package server

import (
	"encoding/json"

	"github.com/yansircc/llm-broker/internal/domain"
)

type accountProjection struct {
	storedPriority    int
	effectivePriority int
	autoScore         int
	windows           []UtilizationWindowResponse
	probeLabel        string
	providerFields    []AccountFieldResponse
}

func (s *Server) projectAccount(acct *domain.Account) accountProjection {
	proj := accountProjection{
		storedPriority:    acct.Priority,
		effectivePriority: acct.Priority,
		windows:           []UtilizationWindowResponse{},
		probeLabel:        string(acct.Provider),
		providerFields:    []AccountFieldResponse{},
	}

	drv, ok := s.drivers[acct.Provider]
	if !ok {
		return proj
	}

	state := json.RawMessage(acct.ProviderStateJSON)
	proj.windows = toWindowResponses(drv.GetUtilization(state))
	proj.probeLabel = drv.Info().ProbeLabel
	proj.providerFields = toFieldResponses(drv.DescribeAccount(acct))

	if acct.PriorityMode == "auto" {
		proj.autoScore = drv.AutoPriority(state)
		proj.effectivePriority = proj.autoScore
	}

	return proj
}
