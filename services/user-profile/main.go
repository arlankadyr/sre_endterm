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
        prometheus.CounterOpts{Name: "userprofile_http_requests_total", Help: "Total requests"},
        []string{"method", "endpoint", "status"},
    )
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "userprofile_http_request_duration_seconds", Help: "Duration"},
        []string{"method", "endpoint"},
    )
)

type Profile struct {
    UserID    uint   `gorm:"primaryKey"`
    FullName  string `json:"full_name"`
    Address   string `json:"address"`
    Phone     string `json:"phone"`
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
    db.AutoMigrate(&Profile{})

    r := mux.NewRouter()
    r.HandleFunc("/profiles/{userId}", getProfile).Methods("GET")
    r.HandleFunc("/profiles/{userId}", updateProfile).Methods("PUT")
    r.Handle("/metrics", promhttp.Handler())
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    r.Use(metricsMiddleware)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8006"
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

func getProfile(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    uid, _ := strconv.Atoi(vars["userId"])
    var profile Profile
    if err := db.First(&profile, uid).Error; err != nil {
        http.Error(w, "Profile not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(profile)
}

func updateProfile(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    uid, _ := strconv.Atoi(vars["userId"])
    var input Profile
    json.NewDecoder(r.Body).Decode(&input)
    input.UserID = uint(uid)
    db.Save(&input)
    json.NewEncoder(w).Encode(input)
}