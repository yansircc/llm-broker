package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
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
	p, err := pool.New(ms, bus, driver.ErrorPauses{
		Pause401:        30 * time.Minute,
		Pause401Refresh: 30 * time.Second,
		Pause403:        10 * time.Minute,
		Pause429:        60 * time.Second,
		Pause529:        5 * time.Minute,
	})
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
	srv.pool, _ = pool.New(srv.store, srv.bus, driver.ErrorPauses{
		Pause401: 30 * time.Minute, Pause401Refresh: 30 * time.Second,
		Pause403: 10 * time.Minute, Pause429: 60 * time.Second, Pause529: 5 * time.Minute,
	})

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
	srv.pool, _ = pool.New(srv.store, srv.bus, driver.ErrorPauses{
		Pause401: 30 * time.Minute, Pause401Refresh: 30 * time.Second,
		Pause403: 10 * time.Minute, Pause429: 60 * time.Second, Pause529: 5 * time.Minute,
	})

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
	srv.pool, _ = pool.New(srv.store, srv.bus, driver.ErrorPauses{
		Pause401: 30 * time.Minute, Pause401Refresh: 30 * time.Second,
		Pause403: 10 * time.Minute, Pause429: 60 * time.Second, Pause529: 5 * time.Minute,
	})

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
