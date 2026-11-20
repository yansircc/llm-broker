package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func quotaProjectionServer(t *testing.T) *Server {
	t.Helper()
	srv := newTestServer(t)
	d := driver.NewClaudeDriver(driver.ClaudeConfig{}, driver.NoopStainlessStore{}, 4)
	srv.pool.SetDrivers(map[domain.Provider]driver.SchedulerDriver{domain.ProviderClaude: d})
	srv.adminDrivers = map[domain.Provider]driver.AdminDriver{domain.ProviderClaude: d}
	srv.catalogDrivers = map[domain.Provider]driver.Descriptor{domain.ProviderClaude: d}
	srv.authMw = auth.NewMiddleware("quota-local-test", srv.store)
	now := time.Now()
	reset := now.Add(24 * time.Hour).Unix()
	for _, a := range []*domain.Account{
		{ID: "fable-limited", Subject: "test-fable", Email: "fable@example.test", Provider: domain.ProviderClaude, Status: domain.StatusActive, PriorityMode: "auto", CreatedAt: now, ProviderStateJSON: `{}`},
		{ID: "five-hour-only", Subject: "test-five", Email: "five@example.test", Provider: domain.ProviderClaude, Status: domain.StatusActive, CreatedAt: now.Add(time.Second), ProviderStateJSON: fmt.Sprintf(`{"five_hour_util":0.25,"five_hour_reset":%d}`, reset)},
		{ID: "expired-window", Subject: "test-expired", Email: "expired@example.test", Provider: domain.ProviderClaude, Status: domain.StatusActive, CreatedAt: now.Add(2 * time.Second), ProviderStateJSON: fmt.Sprintf(`{"seven_day_util":0.66,"seven_day_reset":%d}`, now.Add(-time.Hour).Unix())},
	} {
		if err := srv.pool.Add(a); err != nil {
			t.Fatal(err)
		}
	}
	h := http.Header{}
	for k, v := range map[string]string{"7d-status": "allowed", "7d-utilization": "0.66", "7d-reset": fmt.Sprint(reset), "7d_oi-status": "rejected", "7d_oi-utilization": "1.01", "7d_oi-reset": fmt.Sprint(reset), "overage-status": "rejected", "overage-disabled-reason": "out_of_credits", "representative-claim": "seven_day_overage_included"} {
		h.Set("anthropic-ratelimit-unified-"+k, v)
	}
	h.Set("Retry-After", "86400")
	srv.pool.Observe("fable-limited", d.Interpret(429, h, nil, "claude-fable-5-2", nil))
	return srv
}

func TestClaudeQuotaAPIProjection(t *testing.T) {
	srv := quotaProjectionServer(t)
	w := httptest.NewRecorder()
	srv.handleListAccounts(w, adminRequest("GET", "/admin/accounts"))
	var accounts []AccountListItem
	if err := json.Unmarshal(w.Body.Bytes(), &accounts); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range accounts {
		if a.ID == "fable-limited" {
			found = true
			if a.CooldownUntil != nil || !a.AvailableNative {
				t.Fatalf("model limit displayed as account unavailable: %+v", a)
			}
			if len(a.Windows) != 3 || *a.Windows[0].Pct != 66 || *a.Windows[1].Pct != 100 || a.Windows[1].Scope != "model" || a.Windows[2].Pct != nil {
				t.Fatalf("wrong windows: %+v", a.Windows)
			}
		}
	}
	if !found {
		t.Fatal("missing account")
	}
	w = httptest.NewRecorder()
	srv.handlePublicStatus(w, httptest.NewRequest("GET", "/api/status", nil))
	var status map[string][]publicStatusAccount
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	limits := status["claude"][0].Limits
	if *limits[0].RemainingPct != 34 || *limits[1].RemainingPct != 0 || limits[2].RemainingPct != nil || limits[1].Key != "7d_oi" || limits[1].ObservedAt == 0 {
		t.Fatalf("wrong public limits: %+v", limits)
	}
	if strings.Contains(w.Body.String(), "example.test") {
		t.Fatal("public projection leaked identity")
	}
	expired := status["claude"][2].Limits[0]
	if expired.RemainingPct != nil || expired.Status != "stale" {
		t.Fatalf("expired window not marked unknown: %+v", expired)
	}
}

func TestProbeOptionsRejectBeforeTokenIO(t *testing.T) {
	srv := quotaProjectionServer(t)
	for _, body := range []string{`{"model":"bogus"}`, `{"unexpected":1}`, `{} {}`, `{"model":1}`} {
		r := httptest.NewRequest("POST", "/admin/accounts/fable-limited/test", strings.NewReader(body))
		r.SetPathValue("id", "fable-limited")
		w := httptest.NewRecorder()
		srv.handleTestAccount(w, r)
		if w.Code != 400 {
			t.Errorf("%s => %d", body, w.Code)
		}
	}
}

// Opt-in, loopback-only browser verification uses real projections and built UI.
// End via POST /fixture/finish; the test closes its listener before returning.
func TestClaudeQuotaBrowserFixture(t *testing.T) {
	if os.Getenv("BROKER_QUOTA_BROWSER") != "1" {
		t.Skip("opt-in browser fixture")
	}
	srv := quotaProjectionServer(t)
	mux := http.NewServeMux()
	srv.registerOperationalRoutes(mux)
	srv.registerAdminRoutes(mux)
	srv.mountUIRoutes(mux)
	done := make(chan struct{})
	var once sync.Once
	mux.HandleFunc("POST /fixture/finish", func(w http.ResponseWriter, r *http.Request) { once.Do(func() { close(done) }); w.WriteHeader(204) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	fmt.Printf("QUOTA_BROWSER_URL=http://%s\n", listener.Addr())
	select {
	case <-done:
	case <-time.After(10 * time.Minute):
		t.Fatal("browser fixture timed out")
	}
}

func TestClaudeQuotaRelayKeepsOrdinaryModelsAvailable(t *testing.T) {
	for _, message := range []string{"This request would exceed your account's rate limit.", "Extra usage is required"} {
		t.Run(message, func(t *testing.T) {
			calls := map[string]int{}
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var input struct{ Model string }
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Fatal(err)
				}
				calls[input.Model]++
				h := http.Header{"Content-Type": []string{"application/json"}}
				status := 200
				body := `{"id":"msg-test","type":"message","role":"assistant","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
				if strings.Contains(input.Model, "fable") {
					status = 429
					for k, v := range map[string]string{"7d-status": "allowed", "7d-utilization": "0.66", "7d_oi-status": "rejected", "7d_oi-utilization": "1.01", "7d_oi-reset": fmt.Sprint(time.Now().Add(24 * time.Hour).Unix()), "representative-claim": "seven_day_overage_included"} {
						h.Set("anthropic-ratelimit-unified-"+k, v)
					}
					h.Set("Retry-After", "86400")
					h.Set("Request-Id", "quota-test-request")
					b, _ := json.Marshal(map[string]any{"error": map[string]string{"type": "rate_limit_error", "message": message}})
					body = string(b)
				}
				return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			srv := newCompatTestServer(t, client)
			d := srv.catalogDrivers[domain.ProviderClaude].(driver.ExecutionDriver)
			srv.pool.SetDrivers(map[domain.Provider]driver.SchedulerDriver{domain.ProviderClaude: d})
			// Both public surfaces must obey the same persisted model-specific state.
			request := func(model string, compat bool) *httptest.ResponseRecorder {
				var body, path string
				if compat {
					path = "/compat/v1/chat/completions"
					body = fmt.Sprintf(`{"model":"claude/%s","messages":[{"role":"user","content":"hi"}]}`, model)
				} else {
					path = "/v1/messages"
					body = fmt.Sprintf(`{"model":"%s","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, model)
				}
				r := adminRequest("POST", path)
				r.Body = io.NopCloser(strings.NewReader(body))
				w := httptest.NewRecorder()
				if compat {
					srv.handleCompatOpenAIChatCompletions(w, r)
				} else {
					srv.relay.HandleProviderSurface(domain.ProviderClaude, domain.SurfaceNative)(w, r)
				}
				return w
			}
			if w := request("claude-fable-5-2", false); w.Code != 429 {
				t.Fatalf("first Fable status %d: %s", w.Code, w.Body.String())
			}
			if w := request("claude-opus-5-5", false); w.Code != 200 {
				t.Fatalf("ordinary native status %d: %s", w.Code, w.Body.String())
			}
			if w := request("claude-sonnet-5-5", true); w.Code != 200 {
				t.Fatalf("ordinary compat status %d: %s", w.Code, w.Body.String())
			}
			request("claude-fable-5-2", true)
			if calls["claude-fable-5-2"] != 1 {
				t.Fatalf("Fable retried after scoped cooldown: %+v", calls)
			}
			a := srv.pool.Get("acct-claude-1")
			if a.CooldownUntil != nil {
				t.Fatal("ordinary success left public cooldown")
			}
			found := false
			for _, e := range srv.bus.Recent(20) {
				if e.QuotaKey == "7d_oi" {
					found = true
					if e.CooldownUntil != nil || e.QuotaScope != "model" || e.RequestID != "quota-test-request" {
						t.Fatalf("wrong quota event: %+v", e)
					}
				}
			}
			if !found {
				t.Fatal("quota restriction event missing")
			}
		})
	}
}
