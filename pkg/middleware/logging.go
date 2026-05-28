package middleware

import (
	"database/sql"
	"net"
	"net/http"
	"strings"
	"time"

	"personal-finance-manager/internal/auth"
	"personal-finance-manager/internal/models"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func APILogger(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Milliseconds()
			userID := auth.UserIDFromContext(r.Context())

			go logRequest(db, &models.ApiLog{
				UserID:     nullableUserID(userID),
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: rw.statusCode,
				DurationMs: duration,
				UserAgent:  r.UserAgent(),
				IPAddress:  realIP(r),
			})
		})
	}
}

func logRequest(db *sql.DB, log *models.ApiLog) {
	const q = `
		INSERT INTO api_logs (user_id, method, path, status_code, duration_ms, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, _ = db.Exec(q,
		log.UserID, log.Method, log.Path,
		log.StatusCode, log.DurationMs,
		log.UserAgent, log.IPAddress,
	)
}

func nullableUserID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
