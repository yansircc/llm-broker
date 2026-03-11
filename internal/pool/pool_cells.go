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
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneCell(p.cellLocked(id))
}

func (p *Pool) ListCells() []*domain.EgressCell {
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

func (p *Pool) applyCellCooldownLocked(cell *domain.EgressCell, proposed time.Time) bool {
	if cell == nil {
		return false
	}
	if cell.CooldownUntil != nil && cell.CooldownUntil.After(proposed) {
		return false
	}
	until := proposed.UTC()
	cell.CooldownUntil = &until
	cell.UpdatedAt = time.Now().UTC()
	p.persistCellLocked(cell)
	return true
}

func (p *Pool) CooldownCell(cellID string, proposed time.Time, message string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	cell := p.cellLocked(cellID)
	if !p.applyCellCooldownLocked(cell, proposed) {
		return false
	}
	if message != "" {
		p.bus.Publish(events.Event{
			Type:    events.EventRelayError,
			Message: fmt.Sprintf("cell %s cooldown until %s: %s", cellID, proposed.UTC().Format(time.RFC3339), message),
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
