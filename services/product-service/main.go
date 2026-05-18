package main

import (
    "encoding/json"
    "net/http"
    "os"
    "strconv"
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
        prometheus.CounterOpts{Name: "product_http_requests_total", Help: "Total requests"},
        []string{"method", "endpoint", "status"},
    )
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "product_http_request_duration_seconds", Help: "Duration"},
        []string{"method", "endpoint"},
    )
)

type Product struct {
    ID          uint    `gorm:"primaryKey"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Price       float64 `json:"price"`
    Stock       int     `json:"stock"`
}

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

    r := mux.NewRouter()
    r.HandleFunc("/products", getProducts).Methods("GET")
    r.HandleFunc("/products/{id}", getProduct).Methods("GET")
    r.HandleFunc("/products", createProduct).Methods("POST")
    r.Handle("/metrics", promhttp.Handler())
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    r.Use(metricsMiddleware)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8002"
    }
    log.Infof("Product service on :%s", port)
    log.Fatal(http.ListenAndServe(":"+port, r))
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
    db.Find(&products)
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

func createProduct(w http.ResponseWriter, r *http.Request) {
    var p Product
    json.NewDecoder(r.Body).Decode(&p)
    db.Create(&p)
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(p)
}