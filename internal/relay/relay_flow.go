package relay

import (
	"net/http"

	"github.com/yansircc/llm-broker/internal/driver"
)

func (r *Relay) handleWithDriver(w http.ResponseWriter, req *http.Request, drv driver.ExecutionDriver) {
	ctx := req.Context()

	prepared, handled := r.prepareRelayRequest(w, req, drv)
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
			r.finishRelayFailure(w, drv, prepared.input.IsStream, attempts)
			return
		}
	}

	r.finishRelayFailure(w, drv, prepared.input.IsStream, attempts)
}
