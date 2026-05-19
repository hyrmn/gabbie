// app.js — AlpineJS data and HTMX extensions

document.addEventListener("alpine:init", () => {
    // Toast notification system
    Alpine.data("toasts", () => ({
        items: [],
        push(message, type = "success") {
            const id = Date.now();
            this.items.push({ id, message, type });
            setTimeout(() => this.remove(id), 3000);
        },
        remove(id) {
            this.items = this.items.filter((t) => t.id !== id);
        },
    }));

    // Kanban drag-and-drop state
    Alpine.data("kanban", () => ({
        dragId: null,
        dragStart(ev) {
            this.dragId = ev.target.dataset.itemId;
            ev.dataTransfer.effectAllowed = "move";
        },
        dragOver(ev) {
            ev.preventDefault();
            ev.dataTransfer.dropEffect = "move";
        },
        async drop(ev, status) {
            ev.preventDefault();
            if (!this.dragId) return;
            // HTMX patch to update item status
            const body = new URLSearchParams();
            body.set("status", status);
            await fetch(`/api/items/${this.dragId}/status`, {
                method: "PATCH",
                body: body,
                headers: { "Content-Type": "application/x-www-form-urlencoded" },
            });
            this.dragId = null;
            // Refresh the board via HTMX
            document.body.dispatchEvent(new CustomEvent("kanban:refresh"));
        },
    }));
});

// HTMX: log swap errors for debugging
document.addEventListener("htmx:responseError", (evt) => {
    console.error("HTMX error", evt.detail);
});

document.addEventListener("htmx:sendError", (evt) => {
    console.error("HTMX send error", evt.detail);
});
