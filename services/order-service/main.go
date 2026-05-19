package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

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
	db          *gorm.DB
	dbMu        sync.RWMutex
	broken      bool
	originalDSN string
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "order_http_requests_total", Help: "Total requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "order_http_request_duration_seconds", Help: "Duration"},
		[]string{"method", "endpoint"},
	)
)

type Order struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `json:"user_id"`
	ProductID  uint      `json:"product_id"`
	Quantity   int       `json:"quantity"`
	TotalPrice float64   `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	godotenv.Load()
	originalDSN = os.Getenv("DATABASE_URL")
	if originalDSN == "" {
		originalDSN = "host=postgres user=shopflow password=shopflow dbname=shopflow port=5432 sslmode=disable"
	}
	connectDB(originalDSN)

	r := mux.NewRouter()
	r.HandleFunc("/orders", createOrder).Methods("POST")
	r.HandleFunc("/orders", listOrders).Methods("GET")
	r.HandleFunc("/orders/{id}", getOrder).Methods("GET")
	r.HandleFunc("/orders/{id}", updateOrder).Methods("PUT", "PATCH")
	r.HandleFunc("/orders/{id}", deleteOrder).Methods("DELETE")
	r.HandleFunc("/simulate/failure", simulateFailure).Methods("POST")
	r.HandleFunc("/simulate/recover", simulateRecover).Methods("POST")
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", healthCheck)
	r.Use(metricsMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8003"
	}
	log.Infof("Order service on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func connectDB(dsn string) {
	dbMu.Lock()
	defer dbMu.Unlock()
	var err error
	newDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Error("DB connection error: ", err)
		db = nil
	} else {
		newDB.AutoMigrate(&Order{})
		db = newDB
		log.Info("Connected to database")
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	dbOk := db != nil
	dbMu.RUnlock()
	if dbOk {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "db down"})
	}
}

func simulateFailure(w http.ResponseWriter, r *http.Request) {
	log.Warn("Simulating incident: incorrect database configuration")
	broken = true
	connectDB("host=wronghost user=wrong password=wrong dbname=wrong port=5432 sslmode=disable")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "failure simulated"})
}

func simulateRecover(w http.ResponseWriter, r *http.Request) {
	log.Info("Recovering database connection")
	broken = false
	connectDB(originalDSN)
	json.NewEncoder(w).Encode(map[string]string{"message": "recovered"})
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

func createOrder(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	d := db
	dbMu.RUnlock()
	if d == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if order.Status == "" {
		order.Status = "created"
	}
	d.Create(&order)
	json.NewEncoder(w).Encode(order)
}

func listOrders(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	d := db
	dbMu.RUnlock()
	if d == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	var orders []Order
	query := d.Order("created_at desc")
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&orders).Error; err != nil {
		http.Error(w, "Could not load orders", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(orders)
}

func getOrder(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	d := db
	dbMu.RUnlock()
	if d == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	var order Order
	if err := d.First(&order, id).Error; err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(order)
}

func updateOrder(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	d := db
	dbMu.RUnlock()
	if d == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	var order Order
	if err := d.First(&order, id).Error; err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var input Order
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Status != "" {
		order.Status = input.Status
	}
	if input.Quantity > 0 {
		order.Quantity = input.Quantity
	}
	if input.TotalPrice > 0 {
		order.TotalPrice = input.TotalPrice
	}
	if input.ProductID > 0 {
		order.ProductID = input.ProductID
	}
	d.Save(&order)
	json.NewEncoder(w).Encode(order)
}

func deleteOrder(w http.ResponseWriter, r *http.Request) {
	dbMu.RLock()
	d := db
	dbMu.RUnlock()
	if d == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	result := d.Delete(&Order{}, id)
	if result.RowsAffected == 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
