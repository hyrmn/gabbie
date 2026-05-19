# Team To-Do Tracker — Progress

## Status: Complete ✅ All phases delivered

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

- **Phase 4: Item CRUD** (gabbie-nin) ✅
  - DB queries: CreateItem, CreateItemSimple, GetItemsByList, GetItem, GetItemSimple, GetItemsWithDetails
  - Partial update: UpdateItemPartial with ItemUpdates struct
  - Status cycling: ToggleItemStatus (todo → in_progress → done → todo)
  - Filtering/sorting: ItemFilter with status, assignee, priority, tag, sort_by, sort_dir
  - Handlers: ListItems, CreateItem, UpdateItem, DeleteItem, ToggleItemStatus
  - API handlers: CreateItemJSON, UpdateItemJSON, GetListJSON
  - Templates: _item_list.html, _item_card.html, _new_item_form.html, _status_badge.html
  - Inline editing with form swaps, delete confirmation
  - Filter bar with status, priority, sort controls

- **Phase 4: API & API Keys** (gabbie-pnt) ✅
  - API key generation: crypto/rand + SHA-256 hashing
  - DB queries: CreateAPIKey, GetAPIKey, GetAPIKeysByUser, GetAPIKeyByHash, RevokeAPIKey, UpdateAPIKeyLastUsed
  - API key auth middleware: APIKeyAuthMiddleware (checks Bearer token)
  - EitherAuthMiddleware: tries JWT first, falls back to API key
  - Settings pages: settings.html, settings_api_keys.html
  - Components: _api_key_row.html, _new_key_result.html
  - Handlers: Settings, SettingsAPIKeys, CreateAPIKey, RevokeAPIKey
  - Full REST API: GET/POST /api/lists, GET /api/lists/{id}, POST /api/lists/{id}/items, PUT /api/items/{id}

- **Phase 5: Kanban Board** (gabbie-ac3) ✅
  - Kanban page template (kanban.html) with three columns: Todo, In Progress, Done
  - Compact draggable cards (_kanban_card.html) with priority badges, due dates, assignee initials
  - HTML5 Drag and Drop API for moving items between columns (no external libraries)
  - Inline item creation per column (quick-add form)
  - Visual feedback: dragover highlight (green border), drag ghost (opacity + rotation)
  - Move item API: PUT /api/items/{id}/move with JSON {status: "..."}
  - Handler: KanbanView (GET /lists/{id}/kanban), MoveItem (PUT /items/{id}/move)
  - Working view toggle in list.html (List ↔ Kanban links)
  - `dict` template function added to template engine for flexible context passing
  - 404 and error page handlers (notFound, ServeError)

### Pending
- **Phase 6: Polish & Deployment** (gabbie-8lh) ✅
  - Error pages: error.html (generic), 404.html (not found) — both use base.html layout
  - 404 handler in server.go with HX-Redirect support and JSON fallback for API requests
  - ServeError helper function for consistent server-side error rendering
  - RecoveryMiddleware wraps all routes (page + API) to catch panics and return 500 error pages
  - Spinner component: _spinner.html with Tailwind animate-spin
  - Toast notifications: HX-Trigger headers on all create/update/delete handlers (lists, items, collaborators, API keys)
  - AlpineJS toast bridge: show-toast event listener connects HTMX events to AlpineJS toast store
  - HTMX error handling: responseError handler parses status codes and shows contextual error toasts
  - Global toast auto-dismiss after 4 seconds with animated enter/leave transitions
  - Tailwind CSS: built with `npx @tailwindcss/cli --minify` (real build, not placeholder)
  - Makefile: `css`, `build`, `prod`, `docker`, `test`, `clean` targets
  - Dockerfile: multi-stage build (golang:1.23-alpine + node for Tailwind → alpine runtime), CGO disabled
  - DB_PATH=/data/todotracker.db in Docker, port 8080

### Notes
- Supabase env vars: `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_ANON_KEY`
- Dependencies: `github.com/golang-jwt/jwt/v5`, `github.com/lestrrat-go/jwx/v2`, `github.com/google/uuid`, `modernc.org/sqlite`
- DB_PATH env var controls SQLite file location (default: todotracker.db)
- Tailwind CSS output.css is built (minified, real build via npx @tailwindcss/cli)
- API endpoints accept both JWT session cookies and API key auth (EitherAuthMiddleware)
- Docker: `docker build -t todotracker .` (multi-stage, no CGO, SQLite via modernc.org/sqlite)
- Make targets: `make prod` (minified CSS + static binary), `make docker` (Docker image), `make dev` (air hot reload), `make test`
