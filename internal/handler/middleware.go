package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
	"github.com/google/uuid"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// ObservabilityMiddleware inyecta X-Request-Id (UUID), registra X-Equipo-Consumidor
// y escribe logs estructurados JSON como evidencia de consumo real entre células.
func ObservabilityMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := uuid.New().String()
		w.Header().Set("X-Request-Id", reqID)

		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		equipo := strings.TrimSpace(r.Header.Get("X-Equipo-Consumidor"))
		if equipo == "" {
			equipo = "desconocido"
		}

		logger.Info("http_request",
			slog.String("requestId", reqID),
			slog.String("equipoConsumidor", equipo),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.Int("status", rec.status),
			slog.Int64("duracionMs", time.Since(start).Milliseconds()),
		)
	})
}

// AdmissionSemaphore limita las peticiones concurrentes en endpoints intensivos,
// respondiendo 429 Too Many Requests + Retry-After para proteger la RAM de EC2.
type AdmissionSemaphore struct {
	sem        chan struct{}
	retryAfter int
}

func NewAdmissionSemaphore(maxConcurrent, retryAfterSec int) *AdmissionSemaphore {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if retryAfterSec < 1 {
		retryAfterSec = 1
	}
	return &AdmissionSemaphore{
		sem:        make(chan struct{}, maxConcurrent),
		retryAfter: retryAfterSec,
	}
}

func (a *AdmissionSemaphore) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case a.sem <- struct{}{}:
			defer func() { <-a.sem }()
			next(w, r)
		default:
			w.Header().Set("Retry-After", strconv.Itoa(a.retryAfter))
			writeError(w, r, http.StatusTooManyRequests, "SOBRECARGA",
				"El servicio está atendiendo el máximo de peticiones simultáneas.", nil)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, codigo, mensaje string, detalles []domain.ErrorDetalle) {
	writeJSON(w, status, domain.ErrorResponse{
		Codigo:    codigo,
		Mensaje:   mensaje,
		RequestID: RequestIDFromContext(r.Context()),
		Detalles:  detalles,
	})
}

