package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
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
	// Read the body so we can log it if decoding fails.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_json", "invalid request body")
		return false
	}
	// Log the incoming body (always) to help capture problematic payloads
	// quickly during debugging. This is intentionally verbose but temporary.
	logger.FromCtx(r.Context(), slog.Default()).Info("incoming_json_body",
		"method", r.Method,
		"path", r.URL.Path,
		"content_type", r.Header.Get("Content-Type"),
		"body", string(body),
	)

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// Log the raw body to help debug mismatched JSON shapes.
		// Use the default logger as a safe non-nil base so we don't panic.
		os.WriteFile("bad_json.log", []byte(err.Error() + "\n" + string(body)), 0644)
		logger.FromCtx(r.Context(), slog.Default()).Info("bad_json_body", "error", err.Error(), "body", string(body))
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
