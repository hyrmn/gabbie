// app.js — AlpineJS data and HTMX extensions

document.addEventListener("alpine:init", () => {
    // Toast notification system
    Alpine.data("toasts", () => ({
        items: [],
        push(message, type = "success") {
            const id = Date.now() + Math.random();
            this.items.push({ id, message, type });
            setTimeout(() => this.remove(id), 4000);
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

// HTMX: listen for HX-Trigger headers to show toasts
document.addEventListener("htmx:afterOnLoad", (evt) => {
    // Check for HX-Trigger header
    const triggerHeader = evt.detail.xhr.getAllResponseHeaders();
    const triggerMatch = triggerHeader.match(/hx-trigger:\s*([^\r\n]+)/i);
    if (triggerMatch) {
        try {
            const triggerData = JSON.parse(triggerMatch[1]);
            if (triggerData.showToast) {
                const toastEvent = new CustomEvent("show-toast", {
                    detail: triggerData.showToast,
                });
                document.dispatchEvent(toastEvent);
            }
        } catch (e) {
            // Silently ignore parse errors
        }
    }
});

// HTMX: handle error responses (non-2xx)
document.addEventListener("htmx:responseError", (evt) => {
    const xhr = evt.detail.xhr;
    let message = "Something went wrong";

    if (xhr.status === 400) {
        message = xhr.responseText || "Bad request";
    } else if (xhr.status === 401) {
        message = "Session expired. Please log in again.";
    } else if (xhr.status === 403) {
        message = "You don't have permission to do that.";
    } else if (xhr.status === 404) {
        message = "Resource not found.";
    } else if (xhr.status === 500) {
        message = "Internal server error. Please try again later.";
    } else if (xhr.status === 0) {
        message = "Network error. Check your connection.";
    }

    console.error("HTMX error", evt.detail);

    // Show error toast
    document.dispatchEvent(
        new CustomEvent("show-toast", {
            detail: { message, type: "error" },
        })
    );
});

document.addEventListener("htmx:sendError", (evt) => {
    console.error("HTMX send error", evt.detail);
    document.dispatchEvent(
        new CustomEvent("show-toast", {
            detail: { message: "Network error. Check your connection.", type: "error" },
        })
    );
});

// Global toast listener — bridge between HTMX events and AlpineJS
document.addEventListener("show-toast", (evt) => {
    const { message, type } = evt.detail;
    // Find the toasts Alpine component and call push
    const toastContainer = document.querySelector("[x-data*='toasts']");
    if (toastContainer) {
        const data = Alpine.$data(toastContainer);
        if (data && typeof data.push === "function") {
            data.push(message, type || "success");
        }
    }
});
