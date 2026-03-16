package pool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/events"
)

func cloneLabels(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneProxy(src *domain.ProxyConfig) *domain.ProxyConfig {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func cloneCell(src *domain.EgressCell) *domain.EgressCell {
	if src == nil {
		return nil
	}
	copy := *src
	copy.Proxy = cloneProxy(src.Proxy)
	copy.Labels = cloneLabels(src.Labels)
	return &copy
}

func (p *Pool) cellLocked(cellID string) *domain.EgressCell {
	if cellID == "" {
		return nil
	}
	return p.cells[cellID]
}

func (p *Pool) cellForAccountLocked(acct *domain.Account) *domain.EgressCell {
	if acct == nil || acct.CellID == "" {
		return nil
	}
	return p.cellLocked(acct.CellID)
}

func (p *Pool) cellAvailableLocked(cell *domain.EgressCell, now time.Time) bool {
	if cell == nil {
		return false
	}
	if cell.Status == "" {
		cell.Status = domain.EgressCellActive
	}
	if cell.Status != domain.EgressCellActive {
		return false
	}
	if cell.CooldownUntil != nil && now.Before(*cell.CooldownUntil) {
		return false
	}
	return cell.Proxy != nil && cell.Proxy.Host != "" && cell.Proxy.Port > 0
}

func (p *Pool) GetCell(id string) *domain.EgressCell {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "get_cell", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneCell(p.cellLocked(id))
}

func (p *Pool) ListCells() []*domain.EgressCell {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "list_cells", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*domain.EgressCell, 0, len(p.cells))
	for _, cell := range p.cells {
		result = append(result, cloneCell(cell))
	}
	return result
}

func (p *Pool) SaveCell(cell *domain.EgressCell) error {
	if cell == nil || cell.ID == "" {
		return fmt.Errorf("cell id is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		return fmt.Errorf("refresh pool state: %w", err)
	}

	copy := cloneCell(cell)
	copy.PersistRuntime()
	if copy.StateJSON == "" {
		copy.StateJSON = "{}"
	}
	now := time.Now().UTC()
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = now
	}
	copy.UpdatedAt = now
	if err := p.store.SaveEgressCell(context.Background(), copy); err != nil {
		return err
	}
	copy.HydrateRuntime()
	p.cells[copy.ID] = copy
	return nil
}

func (p *Pool) persistCellLocked(cell *domain.EgressCell) {
	if cell == nil {
		return
	}
	cell.PersistRuntime()
	if cell.StateJSON == "" {
		cell.StateJSON = "{}"
	}
	if cell.CreatedAt.IsZero() {
		cell.CreatedAt = time.Now().UTC()
	}
	if cell.UpdatedAt.IsZero() {
		cell.UpdatedAt = time.Now().UTC()
	}
	if err := p.store.SaveEgressCell(context.Background(), cell); err != nil {
		slog.Error("pool cell persist failed", "cellId", cell.ID, "error", err)
	}
}

func (p *Pool) applyCellCooldownLocked(cell *domain.EgressCell, proposed time.Time) CooldownResult {
	if cell == nil {
		return CooldownResult{}
	}
	if cell.CooldownUntil != nil && cell.CooldownUntil.After(proposed) {
		return CooldownResult{Applied: false, Actual: *cell.CooldownUntil}
	}
	until := proposed.UTC()
	cell.CooldownUntil = &until
	cell.UpdatedAt = time.Now().UTC()
	p.persistCellLocked(cell)
	return CooldownResult{Applied: true, Actual: until}
}

func (p *Pool) CooldownCell(cellID string, proposed time.Time, message string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "cooldown_cell", "cellId", cellID, "error", err)
	}

	cell := p.cellLocked(cellID)
	result := p.applyCellCooldownLocked(cell, proposed)
	if !result.Applied {
		return false
	}
	if message != "" {
		p.bus.Publish(events.Event{
			Type:          events.EventRelayError,
			CellID:        cellID,
			CooldownUntil: &result.Actual,
			Message:       fmt.Sprintf("cell %s cooldown until %s: %s", cellID, result.Actual.Format(time.RFC3339), message),
		})
	}
	slog.Warn("cell cooldown applied", "cellId", cellID, "until", proposed.UTC(), "reason", message)
	return true
}

func (p *Pool) CooldownCellForAccount(accountID string, proposed time.Time, message string) bool {
	p.mu.RLock()
	acct, ok := p.accounts[accountID]
	cellID := ""
	if ok {
		cellID = acct.CellID
	}
	p.mu.RUnlock()
	if cellID == "" {
		return false
	}
	return p.CooldownCell(cellID, proposed, message)
}

func (p *Pool) ClearCellCooldown(cellID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "clear_cell_cooldown", "cellId", cellID, "error", err)
	}

	cell := p.cellLocked(cellID)
	if cell == nil || cell.CooldownUntil == nil {
		return false
	}
	cell.CooldownUntil = nil
	cell.UpdatedAt = time.Now().UTC()
	p.persistCellLocked(cell)
	p.bus.Publish(events.Event{
		Type:    events.EventRecover,
		CellID:  cellID,
		Message: fmt.Sprintf("cell %s cooldown cleared", cellID),
	})
	slog.Info("admin cleared cell cooldown", "cellId", cellID)
	return true
}
