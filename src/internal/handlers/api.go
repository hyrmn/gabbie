// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/models"
)

// ============================
// REST API Handlers (JSON)
// ============================

// GetListJSON returns a single list's details with items as JSON.
func (h *Handler) GetListJSON(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	listID := r.PathValue("id")

	list, err := h.DB.GetList(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch list", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch list")
		return
	}
	if list == nil {
		writeJSONError(w, http.StatusNotFound, "list not found")
		return
	}

	// Fetch items for this list
	items, err := h.DB.GetItemsByListSimple(r.Context(), listID)
	if err != nil {
		h.Logger.Error("failed to fetch items", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch items")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"list":  list,
		"items": items,
	})
}

// CreateItemJSON handles POST /api/lists/{id}/items — create a new item via API.
func (h *Handler) CreateItemJSON(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	listID := r.PathValue("id")

	// Check user has access to the list
	list, err := h.DB.GetList(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch list", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch list")
		return
	}
	if list == nil {
		writeJSONError(w, http.StatusNotFound, "list not found")
		return
	}

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		AssigneeID  *string `json:"assignee_id"`
		Priority    int     `json:"priority"`
		DueDate     *string `json:"due_date"`
		Tags        []string `json:"tags"`
		SortOrder   int     `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSONError(w, http.StatusBadRequest, "title is required")
		return
	}

	// Validate status
	status := models.ItemStatus(req.Status)
	if status == "" {
		status = models.StatusTodo
	}
	if status != models.StatusTodo && status != models.StatusInProgress && status != models.StatusDone {
		writeJSONError(w, http.StatusBadRequest, "invalid status")
		return
	}

	// Validate priority range
	if req.Priority < 0 || req.Priority > 3 {
		writeJSONError(w, http.StatusBadRequest, "priority must be between 0 and 3")
		return
	}

	item, err := h.DB.CreateItem(r.Context(), listID, req.Title, req.Description, status, req.AssigneeID, req.Priority, req.DueDate, req.Tags, req.SortOrder)
	if err != nil {
		h.Logger.Error("failed to create item", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to create item")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

// UpdateItemJSON handles PUT /api/items/{id} — update an item via API.
func (h *Handler) UpdateItemJSON(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	itemID := r.PathValue("id")

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		AssigneeID  *string `json:"assignee_id"`
		Priority    int     `json:"priority"`
		DueDate     *string `json:"due_date"`
		Tags        []string `json:"tags"`
		SortOrder   int     `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate status if provided
	if req.Status != "" {
		status := models.ItemStatus(req.Status)
		if status != models.StatusTodo && status != models.StatusInProgress && status != models.StatusDone {
			writeJSONError(w, http.StatusBadRequest, "invalid status")
			return
		}
	}

	item, err := h.DB.UpdateItem(r.Context(), itemID, user.ID, req.Title, req.Description, req.Status, req.AssigneeID, req.Priority, req.DueDate, req.Tags, req.SortOrder)
	if err != nil {
		h.Logger.Error("failed to update item", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to update item")
		return
	}
	if item == nil {
		writeJSONError(w, http.StatusNotFound, "item not found or no access")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// ============================
// Settings Page Handlers
// ============================

// Settings renders the user settings page.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.Tmpl.Render(w, http.StatusOK, "settings.html", map[string]any{
		"Title": "Settings",
		"User":  user,
	})
}

// SettingsAPIKeys renders the API key management page.
func (h *Handler) SettingsAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	keys, err := h.DB.GetAPIKeysByUser(r.Context(), user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch api keys", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Tmpl.Render(w, http.StatusOK, "settings_api_keys.html", map[string]any{
		"Title": "API Keys — Settings",
		"User":  user,
		"Keys":  keys,
	})
}

// CreateAPIKey handles POST /settings/api-keys — generate a new API key.
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.Logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "Unnamed Key"
	}

	// Generate the key
	prefix, rawKey, hash := db.GenerateAPIKey()

	// Store in DB
	apiKey, err := h.DB.CreateAPIKey(r.Context(), user.ID, name, hash, prefix)
	if err != nil {
		h.Logger.Error("failed to create api key", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// For HTMX requests, render the new key result component
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "API key created", "success")
		h.Tmpl.Render(w, http.StatusOK, "components/_new_key_result.html", map[string]any{
			"APIKey": apiKey,
			"RawKey": rawKey,
		})
		return
	}

	// Full page redirect — show result on the API keys page
	toastTrigger(w, "API key created", "success")
	h.Tmpl.Render(w, http.StatusOK, "settings_api_keys.html", map[string]any{
		"Title": "API Keys — Settings",
		"User":  user,
		"Keys":  []models.APIKey{*apiKey},
	})
}

// RevokeAPIKey handles DELETE /settings/api-keys/{id} — revoke an API key.
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keyID := r.PathValue("id")

	if err := h.DB.RevokeAPIKey(r.Context(), keyID, user.ID); err != nil {
		h.Logger.Error("failed to revoke api key", "error", err)
		if r.Header.Get("HX-Request") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Failed to revoke key"))
			return
		}
		http.Error(w, "Failed to revoke key", http.StatusBadRequest)
		return
	}

	// For HTMX, re-fetch and render the updated key list
	if r.Header.Get("HX-Request") != "" {
		keys, _ := h.DB.GetAPIKeysByUser(r.Context(), user.ID)
		toastTrigger(w, "API key revoked", "info")
		h.Tmpl.Render(w, http.StatusOK, "components/_api_key_row.html", map[string]any{
			"Keys": keys,
		})
		return
	}

	toastTrigger(w, "API key revoked", "info")
	http.Redirect(w, r, "/settings/api-keys", http.StatusSeeOther)
}
