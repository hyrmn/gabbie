// Package handlers provides HTTP handlers for the todotracker application.
package handlers

import (
	"html"
	"net/http"
	"strings"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/models"
)

// Dashboard renders the user's dashboard with all their lists.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	lists, err := h.DB.GetListsByUser(r.Context(), user.ID)
	if err != nil {
		h.Logger.Error("failed to fetch lists", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build list details with owner flags
	type ListDetail struct {
		Name        string
		Description string
		ID          string
		IsOwner     bool
	}

	var listDetails []ListDetail
	for _, l := range lists {
		listDetails = append(listDetails, ListDetail{
			ID:          l.ID,
			Name:        l.Name,
			Description: l.Description,
			IsOwner:     l.OwnerID == user.ID,
		})
	}

	h.Tmpl.Render(w, http.StatusOK, "dashboard.html", map[string]any{
		"Title":  "Dashboard",
		"Lists":  listDetails,
		"UserID": user.ID,
		"User":   user,
	})
}

// ListCreate handles creating a new list via form POST.
func (h *Handler) ListCreate(w http.ResponseWriter, r *http.Request) {
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
		if r.Header.Get("HX-Request") != "" {
			w.Header().Set("HX-Retarget", "#create-list-error")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("List name is required"))
			return
		}
		http.Error(w, "List name is required", http.StatusBadRequest)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))

	list, err := h.DB.CreateList(r.Context(), name, description, user.ID)
	if err != nil {
		h.Logger.Error("failed to create list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Also add owner as collaborator with role 'owner'
	if err := h.DB.AddCollaborator(r.Context(), list.ID, user.ID, string(models.RoleOwner)); err != nil {
		h.Logger.Warn("failed to add owner as collaborator", "error", err)
	}

	// HTMX redirect to the new list
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "List created", "success")
		w.Header().Set("HX-Redirect", "/lists/"+list.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	toastTrigger(w, "List created", "success")
	http.Redirect(w, r, "/lists/"+list.ID, http.StatusSeeOther)
}

// ListView renders a single list with its details, collaborators, and items.
func (h *Handler) ListView(w http.ResponseWriter, r *http.Request) {
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

	collabs, err := h.DB.GetCollaborators(r.Context(), listID)
	if err != nil {
		h.Logger.Error("failed to fetch collaborators", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch items with details
	items, err := h.DB.GetItemsWithDetails(r.Context(), listID, db.ItemFilter{})
	if err != nil {
		h.Logger.Error("failed to fetch items", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch user's lists for sidebar
	sidebarList := listDetailsFromLists(r.Context(), h.DB, user.ID)

	h.Tmpl.Render(w, http.StatusOK, "list.html", map[string]any{
		"Title":         list.Name,
		"List":          list,
		"Lists":         sidebarList,
		"Collaborators": collabs,
		"Items":         items,
		"Filter":        db.ItemFilter{},
		"IsOwner":       list.OwnerID == user.ID,
		"UserID":        user.ID,
		"User":          user,
	})
}

// ListUpdate handles renaming a list via PUT/POST (HTMX inline edit).
func (h *Handler) ListUpdate(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")

	// Check ownership
	isOwner, err := h.DB.IsListOwner(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to check list ownership", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !isOwner {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.Logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		if r.Header.Get("HX-Request") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("List name is required"))
			return
		}
		http.Error(w, "List name is required", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateList(r.Context(), listID, name, description); err != nil {
		h.Logger.Error("failed to update list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated header snippet via HTMX
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "List updated", "success")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<h2 class="text-2xl font-bold text-gray-900">` + html.EscapeString(name) + `</h2>`))
		return
	}

	toastTrigger(w, "List updated", "success")
	http.Redirect(w, r, "/lists/"+listID, http.StatusSeeOther)
}

// ListDelete handles deleting a list.
func (h *Handler) ListDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")

	if err := h.DB.DeleteList(r.Context(), listID, user.ID); err != nil {
		h.Logger.Error("failed to delete list", "error", err)
		if r.Header.Get("HX-Request") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Failed to delete list"))
			return
		}
		http.Error(w, "Failed to delete list", http.StatusBadRequest)
		return
	}

	// HTMX: remove the list card and redirect to dashboard
	if r.Header.Get("HX-Request") != "" {
		toastTrigger(w, "List deleted", "success")
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	toastTrigger(w, "List deleted", "success")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// AddCollaborator handles inviting a user to a list by email.
func (h *Handler) AddCollaborator(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")

	// Check ownership — only owner can add collaborators
	isOwner, err := h.DB.IsListOwner(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to check list ownership", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !isOwner {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.Logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		if r.Header.Get("HX-Request") != "" {
			w.Header().Set("HX-Retarget", "#add-collaborator-error")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Email is required"))
			return
		}
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Look up the user by email
	targetUser, err := h.DB.GetUserByEmail(r.Context(), email)
	if err != nil {
		h.Logger.Error("failed to look up user", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if targetUser == nil {
		if r.Header.Get("HX-Request") != "" {
			w.Header().Set("HX-Retarget", "#add-collaborator-error")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("User not found. They may need to sign in first."))
			return
		}
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Add as collaborator
	if err := h.DB.AddCollaborator(r.Context(), listID, targetUser.ID, string(models.RoleMember)); err != nil {
		h.Logger.Error("failed to add collaborator", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated collaborator list via HTMX
	if r.Header.Get("HX-Request") != "" {
		// Re-fetch collaborators and render
		collabs, _ := h.DB.GetCollaborators(r.Context(), listID)
		toastTrigger(w, "Collaborator added", "success")
		h.Tmpl.Render(w, http.StatusOK, "components/_collaborator_list.html", map[string]any{
			"List":          struct{ ID string }{ID: listID},
			"Collaborators": collabs,
			"IsOwner":       true,
			"UserID":        user.ID,
		})
		return
	}

	toastTrigger(w, "Collaborator added", "success")
	http.Redirect(w, r, "/lists/"+listID, http.StatusSeeOther)
}

// RemoveCollaborator handles removing a user from a list.
func (h *Handler) RemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	listID := r.PathValue("id")
	collabUserID := r.PathValue("userId")

	// Check ownership
	isOwner, err := h.DB.IsListOwner(r.Context(), listID, user.ID)
	if err != nil {
		h.Logger.Error("failed to check list ownership", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !isOwner {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.DB.RemoveCollaborator(r.Context(), listID, collabUserID); err != nil {
		h.Logger.Error("failed to remove collaborator", "error", err)
		if r.Header.Get("HX-Request") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Failed to remove collaborator"))
			return
		}
		http.Error(w, "Failed to remove collaborator", http.StatusBadRequest)
		return
	}

	// Return updated collaborator list via HTMX
	if r.Header.Get("HX-Request") != "" {
		collabs, _ := h.DB.GetCollaborators(r.Context(), listID)
		toastTrigger(w, "Collaborator removed", "success")
		h.Tmpl.Render(w, http.StatusOK, "components/_collaborator_list.html", map[string]any{
			"List":          struct{ ID string }{ID: listID},
			"Collaborators": collabs,
			"IsOwner":       true,
			"UserID":        user.ID,
		})
		return
	}

	toastTrigger(w, "Collaborator removed", "success")
	http.Redirect(w, r, "/lists/"+listID, http.StatusSeeOther)
}
