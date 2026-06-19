package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func (s *Server) registerRoutes(mux *http.ServeMux) {
	s.registerRelayRoutes(mux)
	s.registerCustomerRoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerOperationalRoutes(mux)
	s.mountUIRoutes(mux)
}

func (s *Server) registerRelayRoutes(mux *http.ServeMux) {
	auth := s.authMw.Authenticate
	native := func(handler http.Handler) http.Handler {
		return auth(requireSurfaceHandler(domain.SurfaceNative, handler))
	}
	compat := func(handler http.Handler) http.Handler {
		return auth(requireSurfaceHandler(domain.SurfaceCompat, handler))
	}

	mux.Handle("GET /v1/models", native(http.HandlerFunc(s.handleListModels)))
	mux.Handle("GET /compat/v1/models", compat(http.HandlerFunc(s.handleCompatListModels)))
	mux.Handle("POST /v1/chat/completions", compat(http.HandlerFunc(s.handleCompatOpenAIChatCompletions)))
	mux.Handle("POST /compat/v1/chat/completions", compat(http.HandlerFunc(s.handleCompatOpenAIChatCompletions)))

	for _, provider := range sortedProviders(s.catalogDrivers) {
		drv := s.catalogDrivers[provider]
		for _, path := range drv.Info().RelayPaths {
			mux.Handle("POST "+path, native(http.HandlerFunc(s.relay.HandleProviderSurface(provider, domain.SurfaceNative))))
		}
	}
}

func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	admin := func(handler http.HandlerFunc) http.Handler {
		return s.adminAuth(handler)
	}

	mux.Handle("GET /admin/providers", admin(s.handleListProviders))
	mux.Handle("POST /admin/accounts/generate-auth-url", admin(s.handleGenerateAuthURL))
	mux.Handle("POST /admin/accounts/exchange-code", admin(s.handleExchangeCode))
	mux.Handle("GET /admin/accounts", admin(s.handleListAccounts))
	mux.Handle("GET /admin/accounts/{id}", admin(s.handleGetAccount))
	mux.Handle("DELETE /admin/accounts/{id}", admin(s.handleDeleteAccount))
	mux.Handle("POST /admin/accounts/{id}/email", admin(s.handleUpdateAccountEmail))
	mux.Handle("POST /admin/accounts/{id}/status", admin(s.handleUpdateAccountStatus))
	mux.Handle("POST /admin/accounts/{id}/weight", admin(s.handleUpdateAccountWeight))
	mux.Handle("POST /admin/accounts/{id}/cell", admin(s.handleBindAccountCell))
	mux.Handle("POST /admin/accounts/{id}/refresh", admin(s.handleRefreshAccount))
	mux.Handle("POST /admin/accounts/{id}/test", admin(s.handleTestAccount))
	mux.Handle("POST /admin/openai-compatible-accounts", admin(s.handleCreateOpenAICompatibleAccount))
	mux.Handle("POST /admin/openai-compatible-accounts/{id}", admin(s.handleUpdateOpenAICompatibleAccount))
	mux.Handle("GET /admin/egress/cells", admin(s.handleListEgressCells))
	mux.Handle("POST /admin/egress/cells", admin(s.handleUpsertEgressCell))
	mux.Handle("POST /admin/egress/cells/test-proxy", admin(s.handleTestProxy))
	mux.Handle("POST /admin/egress/cells/{id}/clear-cooldown", admin(s.handleClearEgressCellCooldown))
	mux.Handle("POST /admin/egress/cells/{id}/test", admin(s.handleTestEgressCell))

	mux.Handle("DELETE /admin/events", admin(s.handleClearEvents))

	mux.Handle("POST /admin/users", admin(s.handleCreateUser))
	mux.Handle("GET /admin/users", admin(s.handleListUsers))
	mux.Handle("GET /admin/users/total-costs", admin(s.handleListUserTotalCosts))
	mux.Handle("GET /admin/users/{id}", admin(s.handleGetUser))
	mux.Handle("DELETE /admin/users/{id}", admin(s.handleDeleteUser))
	mux.Handle("POST /admin/users/{id}/regenerate", admin(s.handleRegenerateUserToken))
	mux.Handle("POST /admin/users/{id}/status", admin(s.handleUpdateUserStatus))
	mux.Handle("POST /admin/users/{id}/policy", admin(s.handleUpdateUserPolicy))

	mux.Handle("GET /admin/activity", admin(s.handleActivity))
	mux.Handle("GET /admin/billing/summary", admin(s.handleAdminBillingSummary))
	mux.Handle("GET /admin/billing/orders", admin(s.handleAdminBillingOrders))
	mux.Handle("POST /admin/billing/orders/{id}/refresh", admin(s.handleAdminRefreshPaymentOrder))
	mux.Handle("GET /admin/activity/usage", admin(s.handleActivityUsage))
	mux.Handle("GET /admin/dashboard", admin(s.handleDashboard))
	mux.Handle("GET /admin/capacity", admin(s.handleAdminCapacity))
	mux.Handle("POST /admin/drain", admin(s.handleDrain))
	mux.Handle("GET /admin/drain-status", admin(s.handleDrainStatus))
	mux.Handle("GET /admin/health", admin(s.handleHealth))
	mux.Handle("DELETE /admin/sessions/binding/{uuid}", admin(s.handleUnbindSession))
}

func (s *Server) registerCustomerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/public/config", s.handlePublicConfig)
	mux.HandleFunc("GET /api/public/model-prices", s.handlePublicModelPrices)
	mux.HandleFunc("POST /api/auth/register", s.handleCustomerRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleCustomerLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleCustomerLogout)
	mux.HandleFunc("GET /api/me", s.handleCustomerMe)
	mux.HandleFunc("GET /api/keys", s.handleCustomerListKeys)
	mux.HandleFunc("POST /api/keys", s.handleCustomerCreateKey)
	mux.HandleFunc("PATCH /api/keys/{id}", s.handleCustomerUpdateKey)
	mux.HandleFunc("DELETE /api/keys/{id}", s.handleCustomerDeleteKey)
	mux.HandleFunc("GET /api/billing/summary", s.handleCustomerBillingSummary)
	mux.HandleFunc("GET /api/billing/ledger", s.handleCustomerBillingLedger)
	mux.HandleFunc("POST /api/payments/create", s.handleCreatePayment)
	mux.HandleFunc("GET /api/payments/orders", s.handleCustomerPaymentOrders)
	mux.HandleFunc("GET /api/payments/orders/{id}", s.handleCustomerPaymentOrder)
	mux.HandleFunc("POST /api/payments/orders/{id}/refresh", s.handleCustomerRefreshPaymentOrder)
	mux.HandleFunc("GET /api/payments/notify", s.handlePaymentNotify)
	mux.HandleFunc("POST /api/payments/notify", s.handlePaymentNotify)
	mux.HandleFunc("GET /api/referrals", s.handleCustomerReferrals)
	mux.HandleFunc("GET /api/usage", s.handleCustomerUsage)
}

func (s *Server) registerOperationalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/event_logging/batch", s.handleTelemetryBatch)
	mux.HandleFunc("GET /health", s.handleHealthCheck)
	mux.HandleFunc("GET /ready", s.handleReadyCheck)
}

func (s *Server) handleTelemetryBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"error","store":"%s"}`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReadyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.isDraining() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"draining"}`))
		return
	}
	if err := s.store.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"error","store":"%s"}`, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

func (s *Server) handleDrain(w http.ResponseWriter, r *http.Request) {
	s.startDrain()
	slog.Info("server drain enabled", "activeRequests", len(s.snapshotActiveRequests()))
	writeJSON(w, http.StatusOK, s.drainStatusView())
}

func (s *Server) handleDrainStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.drainStatusView())
}

func (s *Server) drainStatusView() map[string]any {
	active := s.snapshotActiveRequests()
	oldestAgeSeconds := 0.0
	now := time.Now()
	for _, req := range active {
		startedRaw, _ := req["started"].(string)
		started, err := time.Parse(time.RFC3339Nano, startedRaw)
		if err != nil {
			continue
		}
		age := now.Sub(started).Seconds()
		if age > oldestAgeSeconds {
			oldestAgeSeconds = age
		}
	}
	return map[string]any{
		"draining":                   s.isDraining(),
		"active_requests":            len(active),
		"oldest_request_age_seconds": oldestAgeSeconds,
		"requests":                   active,
	}
}

type modelsResponse struct {
	Object string         `json:"object"`
	Data   []driver.Model `json:"data"`
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := make([]driver.Model, 0)
	for _, provider := range sortedProviders(s.catalogDrivers) {
		data = append(data, s.catalogDrivers[provider].Models()...)
	}
	json.NewEncoder(w).Encode(modelsResponse{Object: "list", Data: data})
}
