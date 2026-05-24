// Package auth provides Supabase authentication integration.
// It verifies Supabase JWT tokens, manages session cookies,
// and syncs authenticated users into the local database.
package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// contextKey is a custom type to avoid key collisions.
type contextKey string

const (
	// UserKey is the context key for the authenticated user.
	UserKey contextKey = "user"
	// SessionCookieName is the name of the HTTP-only session cookie.
	SessionCookieName = "sb-access-token"
	// SessionDuration is how long a remembered session lasts (7 days).
	SessionDuration = 7 * 24 * time.Hour
	// ShortSessionDuration is how long a non-remembered session lasts (1 hour).
	ShortSessionDuration = 1 * time.Hour
)

// User represents an authenticated user.
type User struct {
	ID    string
	Email string
}

// Config holds Supabase and runtime configuration.
type Config struct {
	SupabaseURL     string
	SupabaseJWTSecret string // JWT signing secret from Supabase dashboard
	SupabaseAnonKey   string // Anon/public key for Supabase JS client
}

// Service manages JWT verification, JWKS caching, and user sync.
type Service struct {
	cfg    Config
	db     *sql.DB
	logger *slog.Logger

	// JWKS cache
	mu       sync.RWMutex
	jwksKeys map[string]interface{} // parsed public keys indexed by kid
	jwksExp  time.Time
}

// New creates a new auth Service.
func New(cfg Config, db *sql.DB, logger *slog.Logger) *Service {
	s := &Service{
		cfg:    cfg,
		db:     db,
		logger: logger,
	}
	return s
}

// StartJWKSRefresh starts a background goroutine that refreshes the JWKS
// every hour. Call this in a goroutine from main.
func (s *Service) StartJWKSRefresh(ctx context.Context) {
	// Initial fetch
	if err := s.refreshJWKS(ctx); err != nil {
		s.logger.Warn("initial JWKS fetch failed, will retry", "error", err)
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.refreshJWKS(ctx); err != nil {
				s.logger.Warn("JWKS refresh failed", "error", err)
			}
		}
	}
}

// refreshJWKS fetches the JWKS from Supabase and caches the public keys.
func (s *Service) refreshJWKS(ctx context.Context) error {
	jwksURL := s.cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return fmt.Errorf("create JWKS request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch failed: status %d", resp.StatusCode)
	}

	// Parse the JWKS using lestrrat-go/jwx
	keySet, err := jwk.ParseReader(resp.Body)
	if err != nil {
		return fmt.Errorf("parse JWKS: %w", err)
	}

	// Build a map of kid -> public key
	keys := make(map[string]interface{})
	for it := keySet.Iterate(ctx); it.Next(ctx); {
		pair := it.Pair()
		kid := pair.Key.(string)
		rawKey := pair.Value.(jwk.Key)

		var pubkey interface{}
		if err := rawKey.Raw(&pubkey); err != nil {
			s.logger.Warn("failed to extract raw key", "kid", kid, "error", err)
			continue
		}
		keys[kid] = pubkey
	}

	s.mu.Lock()
	s.jwksKeys = keys
	s.jwksExp = time.Now().Add(1 * time.Hour)
	s.mu.Unlock()

	s.logger.Info("JWKS refreshed", "key_count", len(keys))
	return nil
}

// keyfunc returns a jwt.Keyfunc that looks up the signing key by kid
// from our JWKS cache.
func (s *Service) keyfunc() func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("token header missing kid")
		}

		pubKey, ok := s.jwksKeys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id: %s", kid)
		}

		return pubKey, nil
	}
}

// VerifyToken validates a Supabase JWT and returns the user info.
// It checks: signature (via JWKS), issuer, audience, and expiry.
func (s *Service) VerifyToken(tokenString string) (*User, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, s.keyfunc(),
		jwt.WithIssuer(s.cfg.SupabaseURL+"/auth/v1"),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing sub claim")
	}

	email, _ := claims["email"].(string)

	return &User{
		ID:    sub,
		Email: email,
	}, nil
}

// SyncUser ensures the authenticated user exists in the local database.
// On first login, it creates a new user row (upsert by ID).
func (s *Service) SyncUser(ctx context.Context, user *User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, display_name, created_at, updated_at)
		VALUES (?, ?, '', datetime('now'), datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			email = excluded.email,
			updated_at = datetime('now')
	`, user.ID, user.Email)
	if err != nil {
		return fmt.Errorf("sync user: %w", err)
	}
	return nil
}

// SetSessionCookie sets the HTTP-only session cookie on the response.
// The duration controls the MaxAge; use ShortSessionDuration for
// "remember me = false" and SessionDuration for "remember me = true".
func SetSessionCookie(w http.ResponseWriter, token string, duration time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie clears the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// GetUser retrieves the authenticated user from the request context.
func GetUser(r *http.Request) *User {
	u, ok := r.Context().Value(UserKey).(*User)
	if !ok {
		return nil
	}
	return u
}

// ContextWithUser returns a new context with the user set.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}

// ExtractToken extracts the JWT token from the Authorization header
// or the session cookie, in that order of preference.
func ExtractToken(r *http.Request) string {
	// 1. Authorization: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
			return auth[len(prefix):]
		}
	}
	// 2. Session cookie
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

// RenderFunc is the function signature for rendering templates.
type RenderFunc func(w http.ResponseWriter, status int, name string, data any)

// Handlers groups the auth HTTP handlers.
type Handlers struct {
	Service *Service
	Render  RenderFunc
}

// NewHandlers creates a Handlers set bound to the auth service.
func (s *Service) NewHandlers(render RenderFunc) *Handlers {
	return &Handlers{Service: s, Render: render}
}

// Login renders the login page.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	h.Render(w, http.StatusOK, "login.html", map[string]any{
		"Title":           "Login",
		"SupabaseURL":     h.Service.cfg.SupabaseURL,
		"SupabaseAnonKey": h.Service.cfg.SupabaseAnonKey,
	})
}

// Callback receives the Supabase access token from the client after
// a successful login, verifies it, syncs the user, and sets the session cookie.
func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AccessToken string `json:"access_token"`
		RememberMe  bool   `json:"remember_me"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.AccessToken == "" {
		http.Error(w, "missing access_token", http.StatusBadRequest)
		return
	}

	user, err := h.Service.VerifyToken(req.AccessToken)
	if err != nil {
		h.Service.logger.Warn("token verification failed", "error", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	if err := h.Service.SyncUser(r.Context(), user); err != nil {
		h.Service.logger.Error("failed to sync user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	duration := ShortSessionDuration
	if req.RememberMe {
		duration = SessionDuration
	}
	SetSessionCookie(w, req.AccessToken, duration)

	// For HTMX redirects
	w.Header().Set("HX-Redirect", "/dashboard")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user_id":    user.ID,
		"user_email": user.Email,
	})
}

// Logout clears the session cookie and redirects to the login page.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ClearSessionCookie(w)

	// If HTMX request, use HX-Redirect; otherwise standard redirect
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Session returns the current user's info as JSON.
func (h *Handlers) Session(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"authenticated": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"authenticated": true,
		"user_id":       user.ID,
		"user_email":    user.Email,
	})
}

// AuthHandlers holds HTTP handlers for auth endpoints.
type AuthHandlers struct {
	Service *Service
}

// Handlers returns the auth HTTP handlers.
func (s *Service) Handlers() *AuthHandlers {
	return &AuthHandlers{Service: s}
}

// Login redirects to Supabase OAuth login.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.Service.cfg.SupabaseURL+"/auth/v1/authorize", http.StatusFound)
}

// Callback handles the OAuth callback from Supabase.
func (h *AuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("access_token")
	if token == "" {
		http.Error(w, "missing access_token", http.StatusBadRequest)
		return
	}

	user, err := h.Service.VerifyToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	if err := h.Service.SyncUser(r.Context(), user); err != nil {
		http.Error(w, "failed to sync user", http.StatusInternalServerError)
		return
	}

	SetSessionCookie(w, token, SessionDuration)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// Logout clears the session cookie.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// Session returns the current user's session info.
func (h *AuthHandlers) Session(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if user == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":%q,"email":%q}`, user.ID, user.Email)
}
