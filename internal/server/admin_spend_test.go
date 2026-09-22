package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

func spendTestServer(t *testing.T) (*Server, *store.MockStore) {
	t.Helper()
	srv := newTestServer(t)
	ms := srv.store.(*store.MockStore)
	ctx := context.Background()

	for _, u := range []*domain.User{
		{ID: "u-1", Name: "frankie", Status: "active"},
		{ID: "u-2", Name: "alice", Status: "active"},
	} {
		if err := ms.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.Name, err)
		}
	}
	return srv, ms
}

func getSpend(t *testing.T, srv *Server) (spendResponse, []byte) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.handleSpend(w, adminRequest("GET", "/admin/spend"))
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp spendResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body %s)", err, w.Body.String())
	}
	return resp, w.Body.Bytes()
}

func TestSpendEmptyRendersArrayNotNull(t *testing.T) {
	srv, _ := spendTestServer(t)
	resp, body := getSpend(t, srv)
	if len(resp.Users) != 0 {
		t.Fatalf("users = %v, want empty", resp.Users)
	}
	assertJSONArray(t, body, "users")
}

func TestSpendGroupsResolvesNamesAndTotals(t *testing.T) {
	srv, ms := spendTestServer(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, entry := range []*domain.RequestLog{
		{UserID: "u-1", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 10, CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: "u-1", Provider: "codex", Model: "gpt-6", Status: "ok", CostUSD: 5, CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: "u-2", Provider: "claude", Model: "sonnet", Status: "ok", CostUSD: 3, CreatedAt: now.Add(-2 * time.Hour)},
		// Dropped: the synthetic admin id and a since-deleted user are not in `users`.
		{UserID: "admin", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 1000, CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: "u-gone", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 500, CreatedAt: now.Add(-2 * time.Hour)},
	} {
		if _, err := ms.InsertRequestLog(ctx, entry); err != nil {
			t.Fatalf("InsertRequestLog: %v", err)
		}
	}

	resp, _ := getSpend(t, srv)

	if len(resp.Users) != 2 {
		t.Fatalf("got %d user groups, want 2: %+v", len(resp.Users), resp.Users)
	}
	// Sorted by 30d subtotal descending: frankie ($15) before alice ($3).
	if resp.Users[0].UserName != "frankie" || resp.Users[1].UserName != "alice" {
		t.Fatalf("user order = %q, %q; want frankie, alice", resp.Users[0].UserName, resp.Users[1].UserName)
	}
	if got := resp.Users[0].Subtotal.D30; got != 15 {
		t.Errorf("frankie subtotal 30d = %v, want 15", got)
	}
	if got := resp.Total.D30; got != 18 {
		t.Errorf("total 30d = %v, want 18 (admin and deleted users excluded)", got)
	}
	// Models within a group are sorted by 30d spend descending.
	if resp.Users[0].Models[0].Model != "opus" {
		t.Errorf("frankie top model = %q, want opus", resp.Users[0].Models[0].Model)
	}
}

func TestSpendWindowsAreNested(t *testing.T) {
	srv, ms := spendTestServer(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, entry := range []*domain.RequestLog{
		{UserID: "u-1", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 1, CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: "u-1", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 2, CreatedAt: now.Add(-48 * time.Hour)},
		{UserID: "u-1", Provider: "claude", Model: "opus", Status: "ok", CostUSD: 4, CreatedAt: now.Add(-20 * 24 * time.Hour)},
	} {
		if _, err := ms.InsertRequestLog(ctx, entry); err != nil {
			t.Fatalf("InsertRequestLog: %v", err)
		}
	}

	resp, _ := getSpend(t, srv)
	cost := resp.Users[0].Models[0].Cost
	if cost.D1 != 1 || cost.D3 != 3 || cost.D7 != 3 || cost.D30 != 7 {
		t.Fatalf("windows = %+v, want {1 3 3 7}", cost)
	}
	if !(cost.D1 <= cost.D3 && cost.D3 <= cost.D7 && cost.D7 <= cost.D30) {
		t.Fatalf("rolling windows must be nested, got %+v", cost)
	}
}

// The page is behind the login, so the route must reject an anonymous caller
// and a valid non-admin user token alike.
func TestSpendRoute_RequiresAdmin(t *testing.T) {
	srv, ms := spendTestServer(t)
	srv.authMw = auth.NewMiddleware("admin-secret", srv.store)
	if err := ms.CreateUser(context.Background(), &domain.User{
		ID:          "u-relay",
		Name:        "relay-user",
		TokenHash:   tokenHash("relay-token"),
		TokenPrefix: "tk_relay_abcd...",
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	srv.registerAdminRoutes(mux)

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"non-admin user token", "relay-token", http.StatusForbidden},
		{"admin token", "admin-secret", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/spend", nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body %s)", tc.name, w.Code, tc.want, w.Body.String())
		}
	}
}
