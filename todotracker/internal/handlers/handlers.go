// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"log/slog"
	"net/http"

	"github.com/hyrmn/todotracker/internal/templates"
)

// Handler holds dependencies for HTTP request handlers.
type Handler struct {
	Tmpl   *templates.Engine
	Logger *slog.Logger
}

// New creates a new Handler.
func New(tmpl *templates.Engine, logger *slog.Logger) *Handler {
	return &Handler{
		Tmpl:   tmpl,
		Logger: logger,
	}
}

// Index renders the landing page.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.Tmpl.Render(w, http.StatusOK, "index.html", map[string]any{
		"Title": "To-Do Tracker",
	})
}

// Dashboard renders the user's dashboard.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// TODO: fetch user's lists from DB
	h.Tmpl.Render(w, http.StatusOK, "index.html", map[string]any{
		"Title": "Dashboard",
	})
}
