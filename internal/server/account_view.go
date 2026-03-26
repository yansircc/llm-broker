package server

import (
	"encoding/json"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

type accountProjection struct {
	effectiveWeight int
	autoWeight      int
	windows         []UtilizationWindowResponse
	probeLabel      string
	providerFields  []AccountFieldResponse
}

func (s *Server) projectAccount(acct *domain.Account) accountProjection {
	proj := accountProjection{
		effectiveWeight: acct.Priority,
		windows:         []UtilizationWindowResponse{},
		probeLabel:      string(acct.Provider),
		providerFields:  []AccountFieldResponse{},
	}

	drv, ok := s.adminDrivers[acct.Provider]
	if !ok {
		return proj
	}

	state := json.RawMessage(acct.ProviderStateJSON)
	proj.windows = toWindowResponses(drv.GetUtilization(state))
	proj.probeLabel = drv.Info().ProbeLabel
	proj.providerFields = toFieldResponses(drv.DescribeAccount(acct))

	if acct.PriorityMode == "auto" {
		proj.autoWeight = drv.AutoPriority(state)
		proj.effectiveWeight = proj.autoWeight
	}

	return proj
}

func toWindowResponses(windows []driver.UtilWindow) []UtilizationWindowResponse {
	if len(windows) == 0 {
		return []UtilizationWindowResponse{}
	}
	resp := make([]UtilizationWindowResponse, 0, len(windows))
	for _, window := range windows {
		w := UtilizationWindowResponse{
			Label: window.Label,
			Pct:   window.Pct,
			Reset: window.Reset,
		}
		if window.SubLabel != "" {
			w.SubLabel = window.SubLabel
			w.SubPct = &window.SubPct
			w.SubReset = window.SubReset
		}
		resp = append(resp, w)
	}
	return resp
}

func toFieldResponses(fields []driver.AccountField) []AccountFieldResponse {
	if len(fields) == 0 {
		return []AccountFieldResponse{}
	}
	resp := make([]AccountFieldResponse, 0, len(fields))
	for _, field := range fields {
		if field.Label == "" || field.Value == "" {
			continue
		}
		resp = append(resp, AccountFieldResponse{
			Label: field.Label,
			Value: field.Value,
		})
	}
	if len(resp) == 0 {
		return []AccountFieldResponse{}
	}
	return resp
}
