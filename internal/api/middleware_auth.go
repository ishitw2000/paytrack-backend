package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/paytrack/backend/internal/auth"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"
const UserEmailContextKey contextKey = "user_email"

// AuthMiddleware enforces JWT authentication on protected routes
func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization format, expected Bearer <token>"})
				return
			}

			tokenStr := parts[1]
			claims, err := auth.ValidateToken(tokenStr, jwtSecret)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}

			// Attach user info to request context
			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailContextKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext retrieves the authenticated user ID from context
func GetUserIDFromContext(ctx context.Context) (uint, bool) {
	val := ctx.Value(UserIDContextKey)
	if val == nil {
		return 0, false
	}
	userID, ok := val.(uint)
	return userID, ok
}
