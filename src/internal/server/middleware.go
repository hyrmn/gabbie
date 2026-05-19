package server

import (
	"log/slog"
	"net/http"

	"github.com/hyrmn/todotracker/internal/auth"
)

// AuthMiddleware verifies the Supabase JWT from the session cookie
// or Authorization header. On success, it sets the user in the
// request context. On failure, it redirects page requests to /login
// and returns 401 JSON for API/HTMX requests.
func AuthMiddleware(svc *auth.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := auth.ExtractToken(r)
			if token == "" {
				redirectToLogin(w, r)
				return
			}

			user, err := svc.VerifyToken(token)
			if err != nil {
				logger.Debug("token verification failed", "error", err, "path", r.URL.Path)
				redirectToLogin(w, r)
				return
			}

			// Sync user into local DB (non-blocking — fire and forget for performance)
			if err := svc.SyncUser(r.Context(), user); err != nil {
				logger.Warn("failed to sync user", "error", err)
			}

			ctx := auth.ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth is like AuthMiddleware but doesn't reject on failure.
// It sets the user in context if a valid token is present, otherwise
// continues without one. Useful for pages that show different content
// for logged-in vs anonymous users.
func OptionalAuth(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := auth.ExtractToken(r)
			if token != "" {
				if user, err := svc.VerifyToken(token); err == nil {
					ctx := auth.ContextWithUser(r.Context(), user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware sets recommended security headers on every response.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// redirectToLogin redirects to the login page, respecting HTMX requests.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	// Check if this looks like an API request (prefixed with /api/)
	if len(r.URL.Path) >= 4 && r.URL.Path[:5] == "/api/" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
		return
	}

	// HTMX boosted request — use HX-Redirect
	if r.Header.Get("HX-Request") != "" || r.Header.Get("HX-Boosted") != "" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
