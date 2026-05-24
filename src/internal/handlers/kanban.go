// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/models"
)

// KanbanView renders the kanban board for a list.
func (h *Handler) KanbanView(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	listID := r.PathValue("id")

	list, err := h.DB.GetList(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if list == nil {
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}

	// Fetch all items for this list (no filter — we need all statuses)
	items, err := h.DB.GetItemsWithDetails(r.Context(), listID, db.ItemFilter{})
	if err != nil {
		h.Logger.Error("failed to fetch items", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch collaborators for assignee dropdown
	collabs, err := h.DB.GetCollaborators(r.Context(), listID)
	if err != nil {
		h.Logger.Error("failed to fetch collaborators", "error", err)
		// Non-fatal — continue without collaborators
		collabs = nil
	}

	// Fetch user's lists for sidebar
	sidebarLists := listDetailsFromLists(r.Context(), h.DB, user.ID)

	h.Tmpl.Render(w, http.StatusOK, "kanban.html", map[string]any{
		"Title":         list.Name + " — Kanban",
		"List":          list,
		"Lists":         sidebarLists,
		"Items":         items,
		"Collaborators": collabs,
		"IsOwner":       list.OwnerID == user.ID,
		"UserID":        user.ID,
		"User":          user,
	})
}

// MoveItem handles moving an item to a new status via PUT.
// Accepts JSON body: {"status": "in_progress"}
func (h *Handler) MoveItem(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	itemID := r.PathValue("id")

	// Fetch the item to verify list access
	item, err := h.DB.GetItemSimple(r.Context(), itemID)
	if err != nil {
		h.Logger.Error("failed to fetch item", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	// Check user has access to the list
	list, err := h.DB.GetList(r.Context(), item.ListID, user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if list == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse request body
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Logger.Error("failed to parse request", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		string(models.StatusTodo):       true,
		string(models.StatusInProgress): true,
		string(models.StatusDone):       true,
	}
	if !validStatuses[req.Status] {
		http.Error(w, `{"error": "invalid status"}`, http.StatusBadRequest)
		return
	}

	// Update the item status
	if err := h.DB.UpdateItemPartial(r.Context(), itemID, db.ItemUpdates{
		Status: &req.Status,
	}); err != nil {
		h.Logger.Error("failed to update item status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return the updated item
	if r.Header.Get("HX-Request") != "" || r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":     itemID,
			"status": req.Status,
		})
		return
	}

	http.Redirect(w, r, "/lists/"+item.ListID+"/kanban", http.StatusSeeOther)
}

// MoveItemAPI is the JSON-only version of MoveItem for the API.
// Accepts both JWT and API key auth.
func (h *Handler) MoveItemAPI(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	itemID := r.PathValue("id")

	// Fetch the item to verify list access
	item, err := h.DB.GetItemSimple(r.Context(), itemID)
	if err != nil {
		h.Logger.Error("failed to fetch item", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch item")
		return
	}
	if item == nil {
		writeJSONError(w, http.StatusNotFound, "item not found")
		return
	}

	// Check user has access to the list
	list, err := h.DB.GetList(r.Context(), item.ListID, user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch list", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch list")
		return
	}
	if list == nil {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Parse request body
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		string(models.StatusTodo):       true,
		string(models.StatusInProgress): true,
		string(models.StatusDone):       true,
	}
	if !validStatuses[req.Status] {
		writeJSONError(w, http.StatusBadRequest, "invalid status: must be todo, in_progress, or done")
		return
	}

	// Update the item status
	if err := h.DB.UpdateItemPartial(r.Context(), itemID, db.ItemUpdates{
		Status: &req.Status,
	}); err != nil {
		h.Logger.Error("failed to update item status", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to update item")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     itemID,
		"status": req.Status,
		"list_id": item.ListID,
	})
}
