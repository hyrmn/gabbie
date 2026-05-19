// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/models"
)

// ListItems renders the items list for a given list, with optional filtering/sorting.
func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")

	// Check user has access to the list
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

	// Build filter from query params
	filter := db.ItemFilter{
		Status:  r.URL.Query().Get("status"),
		Tag:     r.URL.Query().Get("tag"),
		SortBy:  r.URL.Query().Get("sort_by"),
		SortDir: r.URL.Query().Get("sort_dir"),
	}
	if assignee := r.URL.Query().Get("assignee"); assignee != "" {
		filter.AssigneeID = &assignee
	}
	if prio := r.URL.Query().Get("priority"); prio != "" {
		p, err := strconv.Atoi(prio)
		if err == nil {
			filter.Priority = &p
		}
	}

	// Fetch items with details
	items, err := h.DB.GetItemsWithDetails(r.Context(), listID, filter)
	if err != nil {
		h.Logger.Error("failed to fetch items", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch collaborators for assignee dropdown
	collabs, _ := h.DB.GetCollaborators(r.Context(), listID)

	h.Tmpl.Render(w, http.StatusOK, "components/_item_list.html", map[string]any{
		"List":          list,
		"Items":         items,
		"Filter":        filter,
		"Collaborators": collabs,
		"UserID":        user.ID,
		"IsOwner":       list.OwnerID == user.ID,
	})
}

// CreateItem handles creating a new item via form POST.
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")

	// Check user has access to the list
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

	if err := r.ParseForm(); err != nil {
		h.Logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		if r.Header.Get("HX-Request") != "" {
			w.Header().Set("HX-Retarget", "#create-item-error")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Item title is required"))
			return
		}
		http.Error(w, "Item title is required", http.StatusBadRequest)
		return
	}

	_, err = h.DB.CreateItemSimple(r.Context(), listID, title)
	if err != nil {
		h.Logger.Error("failed to create item", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return the refreshed items list via HTMX
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "Item created", "success")
		h.renderItemsList(w, r, listID, user.ID)
		return
	}

	toastTrigger(w, "Item created", "success")
	http.Redirect(w, r, "/lists/"+listID, http.StatusSeeOther)
}

// UpdateItem handles updating an item via PUT.
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
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

	if err := r.ParseForm(); err != nil {
		h.Logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	updates := db.ItemUpdates{}

	if title := strings.TrimSpace(r.FormValue("title")); title != "" {
		updates.Title = &title
	}
	if desc := r.FormValue("description"); desc != "" {
		updates.Description = &desc
	}
	if status := r.FormValue("status"); status != "" {
		updates.Status = &status
	}
	if assignee := r.FormValue("assignee_id"); assignee != "" {
		updates.AssigneeID = &assignee
	}
	if prio := r.FormValue("priority"); prio != "" {
		p, err := strconv.Atoi(prio)
		if err == nil {
			updates.Priority = &p
		}
	}
	if dueDate := r.FormValue("due_date"); dueDate != "" {
		updates.DueDate = &dueDate
	}
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		// Parse comma-separated tags
		var tags []string
		for _, t := range strings.Split(tagsStr, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
		updates.Tags = tags
	}

	if err := h.DB.UpdateItemPartial(r.Context(), itemID, updates); err != nil {
		h.Logger.Error("failed to update item", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return the updated item card via HTMX
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "Item updated", "success")
		h.renderItemsList(w, r, item.ListID, user.ID)
		return
	}

	toastTrigger(w, "Item updated", "success")
	http.Redirect(w, r, "/lists/"+item.ListID, http.StatusSeeOther)
}

// DeleteItem handles deleting an item.
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
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
	if list == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.DB.DeleteItemSimple(r.Context(), itemID); err != nil {
		h.Logger.Error("failed to delete item", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "Item deleted", "success")
		w.WriteHeader(http.StatusOK)
		return
	}

	toastTrigger(w, "Item deleted", "success")
	http.Redirect(w, r, "/lists/"+item.ListID, http.StatusSeeOther)
}

// ToggleItemStatus cycles the item status (todo → in_progress → done → todo).
func (h *Handler) ToggleItemStatus(w http.ResponseWriter, r *http.Request) {
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
	if list == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	newStatus, err := h.DB.ToggleItemStatus(r.Context(), itemID)
	if err != nil {
		h.Logger.Error("failed to toggle item status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		// Return the updated status badge
		toastTrigger(w, "Status updated", "info")
		badge := statusBadgeHTML(models.ItemStatus(newStatus))
		w.Write([]byte(badge))
		return
	}

	toastTrigger(w, "Status updated", "info")
	http.Redirect(w, r, "/lists/"+item.ListID, http.StatusSeeOther)
}

// renderItemsList is a helper that re-renders the items list component.
func (h *Handler) renderItemsList(w http.ResponseWriter, r *http.Request, listID, userID string) {
	list, _ := h.DB.GetList(r.Context(), listID, userID)
	if list == nil {
		return
	}

	items, _ := h.DB.GetItemsWithDetails(r.Context(), listID, db.ItemFilter{})
	collabs, _ := h.DB.GetCollaborators(r.Context(), listID)

	h.Tmpl.Render(w, http.StatusOK, "components/_item_list.html", map[string]any{
		"List":          list,
		"Items":         items,
		"Filter":        db.ItemFilter{},
		"Collaborators": collabs,
		"UserID":        userID,
		"IsOwner":       list.OwnerID == userID,
	})
}

// statusBadgeHTML returns an HTML snippet for a status badge.
func statusBadgeHTML(status models.ItemStatus) string {
	switch status {
	case models.StatusTodo:
		return `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">To Do</span>`
	case models.StatusInProgress:
		return `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">In Progress</span>`
	case models.StatusDone:
		return `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">Done</span>`
	default:
		return `<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">Unknown</span>`
	}
}
