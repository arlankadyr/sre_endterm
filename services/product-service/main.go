package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db        *gorm.DB
	jwtSecret []byte
)

type Product struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url"`
}

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "product_http_requests_total", Help: "Total requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "product_http_request_duration_seconds", Help: "Duration"},
		[]string{"method", "endpoint"},
	)
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)

	godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=postgres user=shopflow password=shopflow dbname=shopflow port=5432 sslmode=disable"
	}
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("DB connection failed: ", err)
	}
	db.AutoMigrate(&Product{})
	seedProducts()

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("supersecretkey")
	}

	r := mux.NewRouter()
	r.HandleFunc("/products", getProducts).Methods("GET")
	r.HandleFunc("/products/{id}", getProduct).Methods("GET")
	r.HandleFunc("/products", authMiddleware(createProduct)).Methods("POST")
	r.HandleFunc("/products/{id}", authMiddleware(updateProduct)).Methods("PUT")
	r.HandleFunc("/products/{id}", authMiddleware(deleteProduct)).Methods("DELETE")

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", healthHandler)
	r.Use(metricsMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8002"
	}
	log.Infof("Product service on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &respWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		dur := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(dur)
	})
}

type respWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *respWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getProducts(w http.ResponseWriter, r *http.Request) {
	var products []Product
	db.Order("id asc").Find(&products)
	json.NewEncoder(w).Encode(products)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	var p Product
	if err := db.First(&p, id).Error; err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokenString == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		// store user info in context
		r = r.WithContext(context.WithValue(r.Context(), "user_id", claims["sub"]))
		r = r.WithContext(context.WithValue(r.Context(), "is_admin", claims["admin"]))
		next(w, r)
	}
}

func isAdminFromContext(r *http.Request) bool {
	admin, ok := r.Context().Value("is_admin").(bool)
	return ok && admin
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	if !isAdminFromContext(r) {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	normalizeProduct(&p)
	db.Create(&p)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func updateProduct(w http.ResponseWriter, r *http.Request) {
	if !isAdminFromContext(r) {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	var p Product
	if err := db.First(&p, id).Error; err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	var input Product
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	p.Name = input.Name
	p.Description = input.Description
	p.Price = input.Price
	p.Stock = input.Stock
	p.Category = input.Category
	p.ImageURL = input.ImageURL
	normalizeProduct(&p)
	db.Save(&p)
	json.NewEncoder(w).Encode(p)
}

func deleteProduct(w http.ResponseWriter, r *http.Request) {
	if !isAdminFromContext(r) {
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	result := db.Delete(&Product{}, id)
	if result.RowsAffected == 0 {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeProduct(p *Product) {
	if p.Category == "" {
		p.Category = "ShopFlow"
	}
	if p.ImageURL == "" {
		p.ImageURL = "https://images.unsplash.com/photo-1523275335684-37898b6baf30?auto=format&fit=crop&w=900&q=80"
	}
}

func seedProducts() {
	var count int64
	db.Model(&Product{}).Count(&count)
	if count > 0 {
		return
	}

	products := []Product{
		{
			Name:        "Urban Jacket",
			Description: "Легкая городская куртка с водоотталкивающей тканью и аккуратной посадкой.",
			Price:       34900,
			Stock:       18,
			Category:    "Одежда",
			ImageURL:    "https://images.unsplash.com/photo-1515886657613-9f3515b0c78f?auto=format&fit=crop&w=900&q=80",
		},
		{
			Name:        "Trail Sneakers",
			Description: "Универсальные кроссовки для прогулок, учебы и повседневных маршрутов.",
			Price:       42900,
			Stock:       24,
			Category:    "Обувь",
			ImageURL:    "https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=900&q=80",
		},
		{
			Name:        "Studio Headphones",
			Description: "Беспроводные наушники с чистым звуком, мягкими амбушюрами и долгой батареей.",
			Price:       59900,
			Stock:       12,
			Category:    "Техника",
			ImageURL:    "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?auto=format&fit=crop&w=900&q=80",
		},
		{
			Name:        "Daily Backpack",
			Description: "Рюкзак с отделением для ноутбука, плотной спинкой и минималистичным дизайном.",
			Price:       27900,
			Stock:       30,
			Category:    "Аксессуары",
			ImageURL:    "https://images.unsplash.com/photo-1553062407-98eeb64c6a62?auto=format&fit=crop&w=900&q=80",
		},
		{
			Name:        "Smart Watch",
			Description: "Часы для уведомлений, спорта и быстрых платежей в лаконичном корпусе.",
			Price:       74900,
			Stock:       9,
			Category:    "Техника",
			ImageURL:    "https://images.unsplash.com/photo-1523275335684-37898b6baf30?auto=format&fit=crop&w=900&q=80",
		},
		{
			Name:        "Minimal Lamp",
			Description: "Настольная лампа с теплым светом и устойчивым металлическим основанием.",
			Price:       18900,
			Stock:       16,
			Category:    "Дом",
			ImageURL:    "https://images.unsplash.com/photo-1507473885765-e6ed057f782c?auto=format&fit=crop&w=900&q=80",
		},
	}

	if err := db.Create(&products).Error; err != nil {
		log.Warn("Failed to seed products: ", err)
	}
}
