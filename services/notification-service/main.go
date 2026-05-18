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
)

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "notification_http_requests_total", Help: "Total requests"},
        []string{"method", "endpoint", "status"},
    )
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "notification_http_request_duration_seconds", Help: "Duration"},
        []string{"method", "endpoint"},
    )
)

func main() {
    log.SetFormatter(&log.JSONFormatter{})
    godotenv.Load()
    r := mux.NewRouter()
    r.HandleFunc("/notify", notify).Methods("POST")
    r.Handle("/metrics", promhttp.Handler())
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    r.Use(metricsMiddleware)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8005"
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

func notify(w http.ResponseWriter, r *http.Request) {
    var msg struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }
    json.NewDecoder(r.Body).Decode(&msg)
    log.Infof("Sending email to %s: %s - %s", msg.To, msg.Subject, msg.Body)
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "sent (simulated)"})
}