package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadyReflectsDrainState(t *testing.T) {
	srv := newTestServer(t)

	resp := httptest.NewRecorder()
	srv.handleReadyCheck(resp, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("ready status = %d body=%s", resp.Code, resp.Body.String())
	}

	srv.startDrain()
	resp = httptest.NewRecorder()
	srv.handleReadyCheck(resp, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("draining ready status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestDrainRejectsBusinessRequestsAndAllowsOperationalRoutes(t *testing.T) {
	srv := newTestServer(t)
	srv.startDrain()

	handler := srv.requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/api/keys", "/v1/models", "/admin/accounts"} {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, want 503", path, resp.Code)
		}
	}

	for _, path := range []string{"/health", "/ready", "/admin/drain", "/admin/drain-status"} {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want 204", path, resp.Code)
		}
	}
}

func TestDrainStatusTracksActiveRequestsUntilCompletion(t *testing.T) {
	srv := newTestServer(t)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	handler := srv.requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	go func() {
		defer close(done)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request handler did not start")
	}

	status := srv.drainStatusView()
	if got := status["active_requests"]; got != 1 {
		t.Fatalf("active_requests = %v, want 1; status=%#v", got, status)
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request handler did not finish")
	}

	status = srv.drainStatusView()
	if got := status["active_requests"]; got != 0 {
		t.Fatalf("active_requests after completion = %v, want 0; status=%#v", got, status)
	}
}

func TestDrainHandlerEnablesDraining(t *testing.T) {
	srv := newTestServer(t)

	resp := httptest.NewRecorder()
	srv.handleDrain(resp, adminRequest(http.MethodPost, "/admin/drain"))
	if resp.Code != http.StatusOK {
		t.Fatalf("drain status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !srv.isDraining() {
		t.Fatal("server did not enter draining state")
	}
}
