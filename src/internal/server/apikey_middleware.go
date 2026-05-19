package server

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
)

// apiKeyContextKey is the context key for API key auth info.
type apiKeyContextKey struct{}

// APIKeyAuthMiddleware verifies requests using an API key from the
// Authorization: Bearer header. On success, it sets the user in the
// request context and updates last_used_at. On failure, it returns 401 JSON.
func APIKeyAuthMiddleware(database *db.DB, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "missing authorization header"}`))
				return
			}

			rawKey := strings.TrimPrefix(authHeader, "Bearer ")

			// Hash the key for lookup
			hashBytes := sha256.Sum256([]byte(rawKey))
			keyHash := hex.EncodeToString(hashBytes[:])

			// Look up the key
			apiKey, err := database.GetAPIKeyByHash(r.Context(), keyHash)
			if err != nil {
				logger.Error("api key lookup failed", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "internal server error"}`))
				return
			}

			if apiKey == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "invalid or revoked api key"}`))
				return
			}

			// Build a user from the API key owner
			user := &auth.User{
				ID:    apiKey.UserID,
				Email: "", // API keys don't carry email
			}

			// Update last_used_at (non-blocking, fire and forget)
			if err := database.UpdateAPIKeyLastUsed(r.Context(), apiKey.ID); err != nil {
				logger.Debug("failed to update api key last_used_at", "error", err)
			}

			ctx := auth.ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EitherAuthMiddleware tries JWT auth first, falls back to API key auth.
// This allows the /api/ endpoints to accept both session cookies and API keys.
func EitherAuthMiddleware(svc *auth.Service, database *db.DB, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try JWT cookie auth first
			token := auth.ExtractToken(r)
			if token != "" {
				user, err := svc.VerifyToken(token)
				if err == nil {
					// Valid JWT — sync user and proceed
					if err := svc.SyncUser(r.Context(), user); err != nil {
						logger.Warn("failed to sync user", "error", err)
					}
					ctx := auth.ContextWithUser(r.Context(), user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				logger.Debug("jwt verification failed, trying api key", "error", err)
			}

			// Fall back to API key auth
			apiKeyHandler := APIKeyAuthMiddleware(database, logger)(next)
			apiKeyHandler.ServeHTTP(w, r)
		})
	}
}
