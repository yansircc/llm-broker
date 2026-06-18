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
	if prepared.admissionRelease != nil {
		defer prepared.admissionRelease()
	}

	drivers := append([]driver.ExecutionDriver{drv}, r.fallbackDriversFor(drv.Provider())...)
	var lastDriver driver.ExecutionDriver
	var lastAttempts *relayAttemptState
	for driverIndex, attemptDriver := range drivers {
		lastDriver = attemptDriver
		attempts := newRelayAttemptState()
		lastAttempts = attempts
		for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
			if ctx.Err() != nil {
				return
			}

			switch r.executeRelayAttempt(ctx, w, attemptDriver, prepared, attempts, attempt) {
			case relayAttemptDone:
				return
			case relayAttemptStop:
				attempt = r.cfg.MaxRetryAccounts + 1
			}
		}
		if driverIndex == len(drivers)-1 || !attempts.AllowFallback {
			break
		}
	}

	r.finishRelayFailure(w, lastDriver, prepared, lastAttempts)
}
