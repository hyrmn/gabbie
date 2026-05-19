# Team To-Do Tracker — Progress

## Status: Phase 2 - Auth Integration Complete ✅

### Completed
- **Phase 1: Scaffolding** (gabbie-t35) ✅
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

### In Progress
- **Phase 2: Database Schema** (gabbie-rob) — still being worked on by parallel agent

### Pending
- **Phase 3: UI Shell** (gabbie-bxa)
- **Phase 3: List Management** (gabbie-rbu)
- **Phase 4: Item CRUD** (gabbie-nin)
- **Phase 5: Kanban Board** (gabbie-ac3)
- **Phase 5: API** (gabbie-pnt)
- **Phase 6: Polish** (gabbie-8lh)

### Notes
- Auth service initialized with `nil` DB in main.go — DB will be wired in when schema task completes
- Supabase env vars: `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_ANON_KEY`
- Dependencies added: `github.com/golang-jwt/jwt/v5`, `github.com/lestrrat-go/jwx/v2`
