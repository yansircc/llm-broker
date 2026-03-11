package server

import "github.com/yansircc/llm-broker/internal/domain"

func toCellSummary(cell *domain.EgressCell, accountCount int) *EgressCellSummaryResponse {
	if cell == nil {
		return nil
	}
	labels := make(map[string]string, len(cell.Labels))
	for k, v := range cell.Labels {
		labels[k] = v
	}
	return &EgressCellSummaryResponse{
		ID:            cell.ID,
		Name:          cell.Name,
		Status:        string(cell.Status),
		Labels:        labels,
		CooldownUntil: cell.CooldownUntil,
		AccountCount:  accountCount,
	}
}

func accountCountsByCell(accounts []*domain.Account) map[string]int {
	counts := make(map[string]int)
	for _, acct := range accounts {
		if acct.CellID != "" {
			counts[acct.CellID]++
		}
	}
	return counts
}
