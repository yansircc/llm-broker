package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/store"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T) *Server {
	t.Helper()
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		cfg:       &config.Config{},
		store:     ms,
		pool:      p,
		bus:       bus,
		version:   "test",
		startTime: time.Now(),
	}
}

func adminRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), auth.KeyInfoKey, &auth.KeyInfo{
		ID: "admin", Name: "admin", IsAdmin: true,
	})
	return r.WithContext(ctx)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// assertJSONArray checks that the JSON value at the given dot-separated path
// is a JSON array ([]), not null.
func assertJSONArray(t *testing.T, body []byte, path string) {
	t.Helper()
	val := jsonPath(t, body, path)
	if val == nil {
		t.Errorf("path %q is null, expected []", path)
		return
	}
	if _, ok := val.([]interface{}); !ok {
		t.Errorf("path %q is %T, expected array", path, val)
	}
}

// assertJSONNullable checks that the JSON value at path is either null or {}.
func assertJSONNullable(t *testing.T, body []byte, path string) {
	t.Helper()
	val := jsonPath(t, body, path)
	if val == nil {
		return // null is fine
	}
	if m, ok := val.(map[string]interface{}); ok && len(m) == 0 {
		return // {} is fine
	}
	// non-empty object is also fine (it's a nullable field with data)
}

// jsonPath navigates a parsed JSON object by dot-separated keys.
func jsonPath(t *testing.T, body []byte, path string) interface{} {
	t.Helper()
	var root interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	parts := strings.Split(path, ".")
	cur := root
	for _, key := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			t.Fatalf("path %q: expected object at %q, got %T", path, key, cur)
		}
		cur = m[key]
	}
	return cur
}

// ---------------------------------------------------------------------------
// Dashboard contract tests
// ---------------------------------------------------------------------------

func TestDashboard_EmptyData(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	srv.handleDashboard(w, adminRequest("GET", "/admin/dashboard"))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.Bytes()

	assertJSONArray(t, body, "usage")
	assertJSONArray(t, body, "accounts")
	assertJSONArray(t, body, "users")
	assertJSONArray(t, body, "events")

	// health must be an object with known keys
	val := jsonPath(t, body, "health")
	h, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("health is %T, expected object", val)
	}
	for _, key := range []string{"sqlite", "uptime", "version"} {
		if _, ok := h[key]; !ok {
			t.Errorf("health missing key %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Account detail contract tests
// ---------------------------------------------------------------------------

func TestGetAccount_EmptySessions(t *testing.T) {
	srv := newTestServer(t)

	// Add an account via pool
	acct := &domain.Account{
		ID:       "test-1",
		Email:    "test@example.com",
		Provider: domain.ProviderClaude,
		Status:   domain.StatusActive,
	}
	srv.store.SaveAccount(context.Background(), acct)
	// Reload pool
	srv.pool, _ = pool.New(srv.store, srv.bus)

	w := httptest.NewRecorder()
	r := adminRequest("GET", "/admin/accounts/test-1")
	r.SetPathValue("id", "test-1")
	srv.handleGetAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()

	// sessions must be [] not null
	assertJSONArray(t, body, "sessions")
}

func TestGetAccount_NullableFields(t *testing.T) {
	srv := newTestServer(t)

	acct := &domain.Account{
		ID:       "test-2",
		Email:    "t2@example.com",
		Provider: domain.ProviderClaude,
		Status:   domain.StatusActive,
	}
	srv.store.SaveAccount(context.Background(), acct)
	srv.pool, _ = pool.New(srv.store, srv.bus)

	w := httptest.NewRecorder()
	r := adminRequest("GET", "/admin/accounts/test-2")
	r.SetPathValue("id", "test-2")
	srv.handleGetAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.Bytes()

	assertJSONArray(t, body, "provider_fields")
	assertJSONNullable(t, body, "stainless")
}

func TestGetAccount_WithIdentity(t *testing.T) {
	srv := newTestServer(t)

	acct := &domain.Account{
		ID:           "test-3",
		Email:        "t3@example.com",
		Provider:     domain.ProviderClaude,
		Status:       domain.StatusActive,
		IdentityJSON: `{"orgUUID":"abc-123"}`,
	}
	srv.store.SaveAccount(context.Background(), acct)
	srv.pool, _ = pool.New(srv.store, srv.bus)

	w := httptest.NewRecorder()
	r := adminRequest("GET", "/admin/accounts/test-3")
	r.SetPathValue("id", "test-3")
	srv.handleGetAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.Bytes()

	val := jsonPath(t, body, "provider_fields")
	fields, ok := val.([]interface{})
	if !ok {
		t.Fatalf("provider_fields is %T, expected array", val)
	}
	if len(fields) != 0 {
		t.Fatalf("provider_fields = %v, want empty for metadata without display mapping", fields)
	}
}

// ---------------------------------------------------------------------------
// User detail contract tests
// ---------------------------------------------------------------------------

func TestGetUser_EmptyArrays(t *testing.T) {
	srv := newTestServer(t)

	user := &domain.User{
		ID:     "u-1",
		Name:   "testuser",
		Status: "active",
	}
	srv.store.CreateUser(context.Background(), user)

	w := httptest.NewRecorder()
	r := adminRequest("GET", "/admin/users/u-1")
	r.SetPathValue("id", "u-1")
	srv.handleGetUser(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.Bytes()

	assertJSONArray(t, body, "usage")
	assertJSONArray(t, body, "model_usage")
	assertJSONArray(t, body, "recent_requests")
}

// ---------------------------------------------------------------------------
// User list contract test
// ---------------------------------------------------------------------------

func TestListUsers_Empty(t *testing.T) {
	srv := newTestServer(t)

	w := httptest.NewRecorder()
	srv.handleListUsers(w, adminRequest("GET", "/admin/users"))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.Bytes()

	// Must be [] not null
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("response is %T, expected array", result)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d elements", len(arr))
	}
}

func TestAdminAccountsRoute_RequiresAdmin(t *testing.T) {
	srv := newTestServer(t)
	srv.authMw = auth.NewMiddleware("admin-secret", srv.store)

	user := &domain.User{
		ID:          "u-1",
		Name:        "alice",
		TokenHash:   tokenHash("user-token"),
		TokenPrefix: "tk_alice_abcd...",
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	if err := srv.store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	srv.registerAdminRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want %d, body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	srv := newTestServer(t)

	w := httptest.NewRecorder()
	req := adminRequest(http.MethodDelete, "/admin/users/missing")
	req.SetPathValue("id", "missing")
	srv.handleDeleteUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want %d, body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestUpdateUserStatus_NotFound(t *testing.T) {
	srv := newTestServer(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/missing/status", strings.NewReader(`{"status":"disabled"}`))
	req.SetPathValue("id", "missing")
	req = req.WithContext(adminRequest(http.MethodPost, "/admin/users/missing/status").Context())
	srv.handleUpdateUserStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want %d, body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}
