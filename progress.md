# Team To-Do Tracker — Progress

## Status: Phase 4 Complete ✅

### Completed
- **Phase 1: Scaffolding** (gabbie-fz8) ✅
  - Go module, directory structure, Tailwind CSS, HTMX+AlpineJS
  - Embedded static assets, Go template layout
  - Basic routes and handlers

- **Phase 2: Supabase Auth Integration** (gabbie-3lc) ✅
  - JWT verification with JWKS fetching and caching
  - Session cookie management (HTTP-only, Secure, SameSite=Lax, 7 days)
  - User sync (auto-create/upsert on first login)
  - Auth middleware: `AuthMiddleware` (required), `OptionalAuth` (permissive)
  - Auth handlers: Login, Callback, Logout, Session
  - Login page with Supabase JS client (email/password + magic link)
  - base.html updated with auth state in sidebar/top nav
  - index.html updated with conditional auth prompt

- **Phase 2: Database Schema** (gabbie-rob) ✅
  - SQLite schema: users, lists, list_collaborators, items, api_keys
  - Versioned migration system (embedded SQL files)
  - Models with helpers (IsOverdue, HasTag, MarshalTags, etc.)
  - DB wiring into main.go, server, and handlers

- **Phase 3: UI Shell and Navigation** (gabbie-bxa) ✅
  - Full responsive sidebar with user's lists (AlpineJS fetch from /api/lists)
  - Top nav with user dropdown menu (settings, API keys, logout)
  - Mobile sidebar with AlpineJS toggle + transition animations
  - Dynamic sidebar populated from DB via JSON endpoint
  - New list modal with HTMX-powered form
  - Toast notification container (AlpineJS toasts data)
  - Avatar-based user menu with dropdown
  - Stats cards and quick actions on dashboard
  - View toggle placeholder block in base layout

- **Phase 3: List Management with Sharing** (gabbie-rbu) ✅
  - DB queries: CreateList, GetListsByUser, GetList, UpdateList, DeleteList, IsListOwner, GetUserByEmail
  - Collaborator queries: GetCollaborators, AddCollaborator, RemoveCollaborator
  - DB helpers: SQLite date parsing (helpers.go)
  - Handlers: Dashboard (with list overview), ListCreate, ListView, ListUpdate, ListDelete
  - Collaborator handlers: AddCollaborator, RemoveCollaborator
  - API handlers: ListsJSON (GET /api/lists), CreateListJSON (POST /api/lists)
  - Templates: dashboard.html, list.html, components (_list_card, _list_form, _collaborator_list)
  - All operations use HTMX for inline editing, real-time updates
  - Only owner can delete lists or manage collaborators
  - Sidebar navigation on all authenticated pages

- **Phase 4: Item CRUD with HTMX** (gabbie-nin) ✅
  - DB: CreateItem, GetItem, UpdateItem, DeleteItem, GetItemsByList (with filtering)
  - DB: GetItemsWithDetails (with assignee info, overdue flags), ToggleItemStatus
  - DB: ItemFilter struct (status, assignee, priority, tag, sort_by, sort_dir)
  - DB: ItemUpdates struct for partial updates
  - Handlers: ListItems (with filter/sort), CreateItem, UpdateItem, DeleteItem, ToggleItemStatus
  - Templates: _item_list.html (filters, sort bar), _item_card.html (inline edit, status toggle)
  - Templates: _new_item_form.html, _status_badge.html
  - list.html updated to embed item list with full CRUD

- **Phase 4: API and API Keys** (gabbie-pnt) ✅
  - DB: GenerateAPIKey, CreateAPIKey, GetAPIKeysByUser, GetAPIKeyByHash, RevokeAPIKey
  - API Key Auth Middleware: extracts Bearer token, hashes, looks up, checks revoked, sets user
  - EitherAuth middleware: tries JWT first, falls back to API key for /api/ routes
  - REST API: GET/POST /api/lists, GET /api/lists/{id}, POST /api/lists/{id}/items, PUT /api/items/{id}
  - Settings pages: /settings (profile), /settings/api-keys (key management)
  - Templates: settings.html, settings_api_keys.html, _api_key_row.html, _new_key_result.html
  - Key format: "ctx7sk-" + 32 hex chars, SHA-256 hash stored, raw shown once

### Pending
- **Phase 5: Kanban Board** (gabbie-ac3) — AlpineJS-powered drag-and-drop kanban board
- **Phase 6: Polish** (gabbie-8lh) — Form validation, error pages, loading states, toasts, production build

### Notes
- Supabase env vars: `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_ANON_KEY`
- Dependencies: `github.com/golang-jwt/jwt/v5`, `github.com/lestrrat-go/jwx/v2`, `github.com/google/uuid`, `modernc.org/sqlite`
- DB_PATH env var controls SQLite file location (default: todotracker.db)
- Tailwind CSS output is a placeholder (needs `make css` for full build)
