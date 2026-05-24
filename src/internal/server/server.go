// Package server provides the HTTP server, router, and middleware.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/handlers"
	"github.com/hyrmn/todotracker/internal/templates"
)

// Server holds the HTTP mux, templates, logger, DB, and auth service.
type Server struct {
	Mux     *http.ServeMux
	Tmpl    *templates.Engine
	Logger  *slog.Logger
	Handler *handlers.Handler
	Auth    *auth.Service
	DB      *db.DB
}

// New creates a new Server instance.
func New(mux *http.ServeMux, tmpl *templates.Engine, logger *slog.Logger, authService *auth.Service, database *db.DB) *Server {
	return &Server{
		Mux:    mux,
		Tmpl:   tmpl,
		Logger: logger,
		Handler: handlers.New(tmpl, logger, database),
		Auth:   authService,
		DB:     database,
	}
}

// RegisterRoutes sets up all HTTP routes.
func (s *Server) RegisterRoutes() {
	// Auth middleware wrappers
	requireAuth := AuthMiddleware(s.Auth, s.Logger)
	requireAuthRecovery := func(h http.Handler) http.Handler {
		return RecoveryMiddleware(requireAuth(h), s.Logger)
	}

	// Public routes (no auth required)
	authHandlers := s.Auth.NewHandlers(s.Tmpl.Render)
	s.Mux.HandleFunc("GET /login", authHandlers.Login)
	s.Mux.HandleFunc("POST /auth/callback", authHandlers.Callback)
	s.Mux.HandleFunc("POST /auth/logout", authHandlers.Logout)
	s.Mux.HandleFunc("GET /auth/session", authHandlers.Session)

	// Root path: public landing page. Shows different content for
	// authenticated vs unauthenticated users.
	s.Mux.Handle("GET /", OptionalAuth(s.Auth)(RecoveryMiddleware(http.HandlerFunc(s.Handler.Index), s.Logger)))

	// Protected page routes (require JWT session)
	s.Mux.Handle("GET /dashboard", requireAuthRecovery(http.HandlerFunc(s.Handler.Dashboard)))
	s.Mux.Handle("POST /lists", requireAuthRecovery(http.HandlerFunc(s.Handler.ListCreate)))
	s.Mux.Handle("GET /lists/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.ListView)))
	s.Mux.Handle("PUT /lists/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.ListUpdate)))
	s.Mux.Handle("DELETE /lists/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.ListDelete)))
	s.Mux.Handle("POST /lists/{id}/collaborators", requireAuthRecovery(http.HandlerFunc(s.Handler.AddCollaborator)))
	s.Mux.Handle("DELETE /lists/{id}/collaborators/{userId}", requireAuthRecovery(http.HandlerFunc(s.Handler.RemoveCollaborator)))

	// Item CRUD routes
	s.Mux.Handle("GET /lists/{id}/items", requireAuthRecovery(http.HandlerFunc(s.Handler.ListItems)))
	s.Mux.Handle("POST /lists/{id}/items", requireAuthRecovery(http.HandlerFunc(s.Handler.CreateItem)))
	s.Mux.Handle("PUT /items/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.UpdateItem)))
	s.Mux.Handle("DELETE /items/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.DeleteItem)))
	s.Mux.Handle("PUT /items/{id}/status", requireAuthRecovery(http.HandlerFunc(s.Handler.ToggleItemStatus)))

	// Kanban routes
	s.Mux.Handle("GET /lists/{id}/kanban", requireAuthRecovery(http.HandlerFunc(s.Handler.KanbanView)))
	s.Mux.Handle("PUT /items/{id}/move", requireAuthRecovery(http.HandlerFunc(s.Handler.MoveItem)))

	// Settings pages
	s.Mux.Handle("GET /settings", requireAuthRecovery(http.HandlerFunc(s.Handler.Settings)))
	s.Mux.Handle("GET /settings/api-keys", requireAuthRecovery(http.HandlerFunc(s.Handler.SettingsAPIKeys)))
	s.Mux.Handle("POST /settings/api-keys", requireAuthRecovery(http.HandlerFunc(s.Handler.CreateAPIKey)))
	s.Mux.Handle("DELETE /settings/api-keys/{id}", requireAuthRecovery(http.HandlerFunc(s.Handler.RevokeAPIKey)))

	// API routes — accept both JWT cookie and API key auth
	apiAuth := EitherAuthMiddleware(s.Auth, s.DB, s.Logger)
	s.Mux.Handle("GET /api/lists", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.ListsJSON)), s.Logger))
	s.Mux.Handle("POST /api/lists", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.CreateListJSON)), s.Logger))
	s.Mux.Handle("GET /api/lists/{id}", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.GetListJSON)), s.Logger))
	s.Mux.Handle("POST /api/lists/{id}/items", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.CreateItemJSON)), s.Logger))
	s.Mux.Handle("PUT /api/items/{id}", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.UpdateItemJSON)), s.Logger))
	s.Mux.Handle("PUT /api/items/{id}/move", RecoveryMiddleware(apiAuth(http.HandlerFunc(s.Handler.MoveItemAPI)), s.Logger))

	// 404 fallback — must be registered last so it doesn't shadow other routes.
	// In Go 1.22, HandleFunc("/", ...) matches GET / and POST / exactly,
	// so explicit routes above take precedence. This only catches the root.
	s.Mux.HandleFunc("/", s.notFound)
}

// LoggingMiddleware logs each request with method, path, duration, and status.
func LoggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.statusCode,
			"duration", time.Since(start).String(),
		)
	})
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// notFound renders the 404 page.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	// For API requests, return JSON
	if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
		return
	}

	// For HTMX requests, redirect to dashboard
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Render the 404 page
	data := map[string]any{
		"Title":      "Page Not Found",
		"StatusCode": 404,
		"Message":    "The page you're looking for doesn't exist or has been moved.",
	}
	s.Tmpl.Render(w, http.StatusNotFound, "404.html", data)
}

// ServeError renders an error page for server-side errors.
func ServeError(w http.ResponseWriter, r *http.Request, tmplEngine *templates.Engine, statusCode int, title string, message string) {
	// For API requests, return JSON
	if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}

	data := map[string]any{
		"Title":      title,
		"StatusCode": statusCode,
		"Message":    message,
	}
	tmplEngine.Render(w, statusCode, "error.html", data)
}
