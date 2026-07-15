package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxParticipantID ctxKey = iota
	ctxGroup
)

const (
	AccessCookie  = "ap_access"
	RefreshCookie = "ap_refresh"
)

// Middleware проверяет access-JWT из http-only cookie и кладёт
// participant_id и группу в контекст запроса.
func Middleware(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(AccessCookie)
			if err != nil {
				unauthorized(w)
				return
			}
			pid, group, err := tm.ParseAccess(cookie.Value)
			if err != nil {
				unauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxParticipantID, pid)
			ctx = context.WithValue(ctx, ctxGroup, group)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"требуется авторизация"}`))
}

// ParticipantID — из контекста, установленного Middleware.
func ParticipantID(ctx context.Context) uuid.UUID {
	pid, _ := ctx.Value(ctxParticipantID).(uuid.UUID)
	return pid
}

// Group — группа участника (A/B) из контекста.
func Group(ctx context.Context) string {
	g, _ := ctx.Value(ctxGroup).(string)
	return g
}
