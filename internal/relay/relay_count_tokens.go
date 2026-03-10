package relay

import (
	"log/slog"
	"net/http"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/driver"
)

func (r *Relay) handleCountTokens(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver, input *driver.RelayInput, keyInfo *auth.KeyInfo) {
	ctx := req.Context()

	acct, err := r.pool.Pick(drv, nil, input.Model, keyInfo.BoundAccountID)
	if err != nil {
		slog.Warn("count_tokens: account selection failed", "error", err)
		drv.WriteError(w, http.StatusServiceUnavailable, "no available accounts")
		return
	}

	accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		slog.Warn("count_tokens: token unavailable", "error", err, "accountId", acct.ID)
		drv.WriteError(w, http.StatusServiceUnavailable, "token unavailable")
		return
	}

	upReq, err := drv.BuildRequest(ctx, input, acct, accessToken)
	if err != nil {
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

	drv.ForwardResponse(w, resp)
}
