package relay

import (
	"net/http"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/requestid"
)

func (r *Relay) handleWithDriver(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver, surface domain.Surface) {
	req = requestid.Ensure(req, w)
	ctx := req.Context()

	prepared, handled := r.prepareRelayRequest(w, req, drv, surface)
	if handled {
		return
	}

	attempts := newRelayAttemptState()
	for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		switch r.executeRelayAttempt(ctx, w, drv, prepared, attempts, attempt) {
		case relayAttemptDone:
			return
		case relayAttemptStop:
			r.finishRelayFailure(w, drv, prepared, attempts)
			return
		}
	}

	r.finishRelayFailure(w, drv, prepared, attempts)
}
