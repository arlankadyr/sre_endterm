package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
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

// User model
type User struct {
	ID        uint   `gorm:"primaryKey"`
	Email     string `gorm:"unique"`
	Password  string
	IsAdmin   bool `gorm:"default:false"`
	CreatedAt time.Time
}

// Prometheus metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "auth_http_requests_total", Help: "Total number of HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "auth_http_request_duration_seconds", Help: "Duration of HTTP requests"},
		[]string{"method", "endpoint"},
	)
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Warn("No .env file found, using system env")
	}

	// Database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=postgres user=shopflow password=shopflow dbname=shopflow port=5432 sslmode=disable"
	}
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	db.AutoMigrate(&User{})

	ensureAdminUser()

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("supersecretkey")
		log.Warn("JWT_SECRET not set, using default")
	}

	r := mux.NewRouter()

	// Public endpoints
	r.HandleFunc("/register", registerHandler).Methods("POST")
	r.HandleFunc("/login", loginHandler).Methods("POST")
	r.HandleFunc("/me", authMiddleware(meHandler)).Methods("GET")

	// Metrics
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", healthHandler)

	r.Use(metricsMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	log.Infof("Auth service listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	user := User{Email: creds.Email, Password: creds.Password} // hash omitted for demo
	result := db.Create(&user)
	if result.Error != nil {
		http.Error(w, "Email already exists", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User created"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var user User
	if err := db.Where("email = ? AND password = ?", creds.Email, creds.Password).First(&user).Error; err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"admin": user.IsAdmin,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
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
		// Store user info in context (optional)
		r = r.WithContext(context.WithValue(r.Context(), "user_id", claims["sub"]))
		r = r.WithContext(context.WithValue(r.Context(), "is_admin", claims["admin"]))
		next(w, r)
	}
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(float64)
	isAdmin := r.Context().Value("is_admin").(bool)
	var user User
	if err := db.First(&user, uint(userID)).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       user.ID,
		"email":    user.Email,
		"is_admin": isAdmin,
	})
}

func ensureAdminUser() {
	var admin User
	if err := db.Where("email = ?", "admin@shop.com").First(&admin).Error; err != nil {
		admin = User{Email: "admin@shop.com", Password: "admin123", IsAdmin: true}
		if err := db.Create(&admin).Error; err != nil {
			log.Warn("Failed to create admin user: ", err)
			return
		}
		log.Info("Created admin user: admin@shop.com / admin123")
		return
	}

	admin.Password = "admin123"
	admin.IsAdmin = true
	if err := db.Save(&admin).Error; err != nil {
		log.Warn("Failed to update admin user: ", err)
	}
}
