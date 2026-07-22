package relay

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/pool"
)

func (r *Relay) handleCountTokens(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver, input *driver.RelayInput, keyInfo *auth.KeyInfo, surface domain.Surface) {
	ctx := req.Context()

	acct, lease, err := r.pool.AcquireRoute(ctx, drv, pool.RouteRequest{
		Model:         input.Model,
		Surface:       surface,
		HardAccountID: keyInfo.BoundAccountID,
	})
	if err != nil {
		slog.Warn("count_tokens: account selection failed", "error", err)
		drv.WriteError(w, http.StatusServiceUnavailable, "no available accounts")
		return
	}
	defer lease.Abort()

	accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		slog.Warn("count_tokens: token unavailable", "error", err, "accountId", acct.ID)
		drv.WriteError(w, http.StatusServiceUnavailable, "token unavailable")
		return
	}

	upReq, err := drv.BuildRequest(ctx, input, acct, accessToken)
	if err != nil {
		var requestErr *driver.RequestValidationError
		if errors.As(err, &requestErr) {
			drv.WriteError(w, requestErr.StatusCode, requestErr.Message)
			return
		}
		drv.WriteError(w, http.StatusInternalServerError, "failed to build request")
		return
	}

	client := r.transport.ClientForAccount(acct)
	resp, err := client.Do(upReq)
	if err != nil {
		slog.Error("count_tokens upstream failed", "error", err)
		drv.WriteError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	if err := lease.Accept(ctx, 0); err != nil {
		slog.Error("count_tokens: accept route failed", "error", err, "accountId", acct.ID)
		drv.WriteError(w, http.StatusServiceUnavailable, "route state unavailable")
		return
	}
	drv.ForwardResponse(w, resp)
	lease.Finish()
}
