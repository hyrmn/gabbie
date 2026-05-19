// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/models"
	"github.com/hyrmn/todotracker/internal/templates"
)

// Handler holds dependencies for HTTP request handlers.
type Handler struct {
	Tmpl   *templates.Engine
	Logger *slog.Logger
	DB     *db.DB
}

// New creates a new Handler.
func New(tmpl *templates.Engine, logger *slog.Logger, database *db.DB) *Handler {
	return &Handler{
		Tmpl:   tmpl,
		Logger: logger,
		DB:     database,
	}
}

// Index renders the landing page.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.Tmpl.Render(w, http.StatusOK, "index.html", map[string]any{
		"Title": "To-Do Tracker",
	})
}

// ListsJSON returns a JSON array of the user's lists for the sidebar.
func (h *Handler) ListsJSON(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	lists, err := h.DB.GetListsByUser(r.Context(), user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch lists", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch lists")
		return
	}

	// Return simplified list objects for the sidebar
	type ListSummary struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	summaries := make([]ListSummary, 0, len(lists))
	for _, l := range lists {
		summaries = append(summaries, ListSummary{ID: l.ID, Name: l.Name})
	}

	writeJSON(w, http.StatusOK, summaries)
}

// CreateListJSON handles POST /api/lists for creating a new list via API.
func (h *Handler) CreateListJSON(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	description := r.FormValue("description")

	list, err := h.DB.CreateList(r.Context(), name, description, user.ID)
	if err != nil {
		h.Logger.Error("failed to create list", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to create list")
		return
	}

	if err := h.DB.AddCollaborator(r.Context(), list.ID, user.ID, string(models.RoleOwner)); err != nil {
		h.Logger.Warn("failed to add owner as collaborator", "error", err)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":   list.ID,
		"name": list.Name,
	})
}

// listDetailsFromLists builds the sidebar list summary for a user.
func listDetailsFromLists(ctx context.Context, database *db.DB, userID string) []map[string]any {
	lists, err := database.GetListsByUser(ctx, userID)
	if err != nil {
		return nil
	}
	details := make([]map[string]any, 0, len(lists))
	for _, l := range lists {
		details = append(details, map[string]any{
			"ID":      l.ID,
			"Name":    l.Name,
			"IsOwner": l.OwnerID == userID,
		})
	}
	return details
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// toastTrigger sets an HX-Trigger response header to show a toast notification.
func toastTrigger(w http.ResponseWriter, message, tpe string) {
	data, _ := json.Marshal(map[string]any{
		"showToast": map[string]string{
			"message": message,
			"type":    tpe,
		},
	})
	w.Header().Set("HX-Trigger", string(data))
}
