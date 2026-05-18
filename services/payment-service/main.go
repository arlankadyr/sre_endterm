package main

import (
    "encoding/json"
    "net/http"
    "os"
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

var db *gorm.DB

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "payment_http_requests_total", Help: "Total requests"},
        []string{"method", "endpoint", "status"},
    )
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "payment_http_request_duration_seconds", Help: "Duration"},
        []string{"method", "endpoint"},
    )
)

type Payment struct {
    ID        uint    `gorm:"primaryKey"`
    OrderID   uint    `json:"order_id"`
    Amount    float64 `json:"amount"`
    Status    string  `json:"status"` // pending, completed, failed
    CreatedAt time.Time
}

func main() {
    log.SetFormatter(&log.JSONFormatter{})
    godotenv.Load()
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        dsn = "host=postgres user=shopflow password=shopflow dbname=shopflow port=5432 sslmode=disable"
    }
    var err error
    db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }
    db.AutoMigrate(&Payment{})

    r := mux.NewRouter()
    r.HandleFunc("/payments", processPayment).Methods("POST")
    r.Handle("/metrics", promhttp.Handler())
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    r.Use(metricsMiddleware)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8004"
    }
    log.Fatal(http.ListenAndServe(":"+port, r))
}

func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &respWriter{ResponseWriter: w, statusCode: 200}
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

func processPayment(w http.ResponseWriter, r *http.Request) {
    var req struct {
        OrderID uint    `json:"order_id"`
        Amount  float64 `json:"amount"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    payment := Payment{
        OrderID: req.OrderID,
        Amount:  req.Amount,
        Status:  "completed",
    }
    db.Create(&payment)
    json.NewEncoder(w).Encode(payment)
}