package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/logger"
)

// ----- response helpers -----

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ErrorResponse is the wire shape for all API errors.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorResponse{Error: msg, Code: code})
}

func WriteValidationError(w http.ResponseWriter, details map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, ErrorResponse{
		Error:   "validation failed",
		Code:    "validation_error",
		Details: details,
	})
}

// ----- domain error mapping -----

type ErrorKind string

const (
	KindNotFound     ErrorKind = "not_found"
	KindConflict     ErrorKind = "conflict"
	KindUnauthorized ErrorKind = "unauthorized"
	KindForbidden    ErrorKind = "forbidden"
	KindValidation   ErrorKind = "validation"
)

type DomainError struct {
	Kind    ErrorKind
	Message string
}

func (e *DomainError) Error() string { return e.Message }

func NewDomainError(kind ErrorKind, msg string) *DomainError {
	return &DomainError{Kind: kind, Message: msg}
}

func WriteDomainError(w http.ResponseWriter, err error) {
	var de *DomainError
	if errors.As(err, &de) {
		switch de.Kind {
		case KindNotFound:
			WriteError(w, http.StatusNotFound, "not_found", de.Message)
		case KindConflict:
			WriteError(w, http.StatusConflict, "conflict", de.Message)
		case KindUnauthorized:
			WriteError(w, http.StatusUnauthorized, "unauthorized", de.Message)
		case KindForbidden:
			WriteError(w, http.StatusForbidden, "forbidden", de.Message)
		case KindValidation:
			WriteError(w, http.StatusBadRequest, "validation", de.Message)
		default:
			WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
}

// ----- middleware -----

// RequestID assigns or propagates an incoming X-Request-ID header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := logger.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AccessLog logs every request as a single structured line.
func AccessLog(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.FromCtx(r.Context(), base).Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
			)
		})
	}
}

// Recoverer turns panics into 500s and logs them.
func Recoverer(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					logger.FromCtx(r.Context(), base).Error("panic", "panic", rv)
					WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// DecodeJSON decodes a request body, returning false (and writing a 400) on failure.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_json", "invalid request body")
		return false
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		logger.FromCtx(r.Context(), slog.Default()).Warn("bad_json_body",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err.Error(),
		)
		WriteError(w, http.StatusBadRequest, "bad_json", "invalid request body")
		return false
	}
	return true
}

// ClientIP extracts the best-effort client IP from common proxy headers.
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// First IP in the list is the original client.
		for i := 0; i < len(v); i++ {
			if v[i] == ',' {
				return v[:i]
			}
		}
		return v
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	return r.RemoteAddr
}

// ContextWithTimeout returns a derived context capped at 30s for handlers.
func ContextWithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 30*time.Second)
}

// SecurityHeaders adds standard security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

// IP-based Rate Limiter (Token Bucket algorithm).
// Limits to 10k requests per minute per IP.
type clientLimiter struct {
	tokens    float64
	lastCheck time.Time
}

var (
	limiters     = make(map[string]*clientLimiter)
	authLimiters = make(map[string]*clientLimiter)
	mu           sync.Mutex
)

// AuthRateLimiter applies a strict 10 requests/minute limit — for login/password endpoints.
func AuthRateLimiter(next http.Handler) http.Handler {
	const (
		ratePerSecond = 10.0 / 60.0
		maxTokens     = 10.0
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r)
		mu.Lock()
		client, exists := authLimiters[ip]
		now := time.Now()
		if !exists {
			client = &clientLimiter{tokens: maxTokens, lastCheck: now}
			authLimiters[ip] = client
		} else {
			elapsed := now.Sub(client.lastCheck).Seconds()
			client.tokens += elapsed * ratePerSecond
			if client.tokens > maxTokens {
				client.tokens = maxTokens
			}
			client.lastCheck = now
		}
		if client.tokens >= 1.0 {
			client.tokens -= 1.0
			mu.Unlock()
			next.ServeHTTP(w, r)
		} else {
			mu.Unlock()
			WriteError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many attempts, please wait before trying again")
		}
	})
}

// RateLimiter limits requests per client IP to a specified rate.
func RateLimiter(next http.Handler) http.Handler {
	const (
		ratePerSecond = 10000.0 / 60.0 // ~166.67 tokens per second (10k requests per minute)
		maxTokens     = 10000.0
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r)

		mu.Lock()
		client, exists := limiters[ip]
		now := time.Now()

		if !exists {
			client = &clientLimiter{
				tokens:    maxTokens,
				lastCheck: now,
			}
			limiters[ip] = client
		} else {
			// Replenish tokens based on elapsed time
			elapsed := now.Sub(client.lastCheck).Seconds()
			client.tokens += elapsed * ratePerSecond
			if client.tokens > maxTokens {
				client.tokens = maxTokens
			}
			client.lastCheck = now
		}

		if client.tokens >= 1.0 {
			client.tokens -= 1.0
			mu.Unlock()
			next.ServeHTTP(w, r)
		} else {
			mu.Unlock()
			WriteError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "rate limit exceeded. Maximum 10,000 requests per minute allowed.")
		}
	})
}

