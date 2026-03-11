package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const egressCellCols = `id, name, status, proxy_json, labels_json, cooldown_until, state_json, created_at, updated_at`

func scanEgressCell(scanner interface{ Scan(...any) error }) (*domain.EgressCell, error) {
	var (
		id, name, status, proxyJSON, labelsJSON, stateJSON string
		createdAt, updatedAt                               int64
		cooldownUntil                                      sql.NullInt64
	)
	if err := scanner.Scan(&id, &name, &status, &proxyJSON, &labelsJSON, &cooldownUntil, &stateJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	cell := &domain.EgressCell{
		ID:            id,
		Name:          name,
		Status:        domain.EgressCellStatus(status),
		ProxyJSON:     proxyJSON,
		LabelsJSON:    labelsJSON,
		CooldownUntil: scanNullableTime(cooldownUntil),
		StateJSON:     stateJSON,
		CreatedAt:     time.Unix(createdAt, 0).UTC(),
		UpdatedAt:     time.Unix(updatedAt, 0).UTC(),
	}
	cell.HydrateRuntime()
	return cell, nil
}

func (s *SQLiteStore) GetEgressCell(ctx context.Context, id string) (*domain.EgressCell, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+egressCellCols+" FROM egress_cells WHERE id = ?", id)
	cell, err := scanEgressCell(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cell, err
}

func (s *SQLiteStore) ListEgressCells(ctx context.Context) ([]*domain.EgressCell, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+egressCellCols+" FROM egress_cells ORDER BY created_at, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []*domain.EgressCell
	for rows.Next() {
		cell, err := scanEgressCell(rows)
		if err != nil {
			return nil, err
		}
		cells = append(cells, cell)
	}
	return cells, rows.Err()
}

func (s *SQLiteStore) SaveEgressCell(ctx context.Context, cell *domain.EgressCell) error {
	cell.PersistRuntime()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO egress_cells (
			id, name, status, proxy_json, labels_json, cooldown_until, state_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			status=excluded.status,
			proxy_json=excluded.proxy_json,
			labels_json=excluded.labels_json,
			cooldown_until=excluded.cooldown_until,
			state_json=excluded.state_json,
			created_at=excluded.created_at,
			updated_at=excluded.updated_at
	`,
		cell.ID,
		cell.Name,
		string(cell.Status),
		cell.ProxyJSON,
		cell.LabelsJSON,
		nullableUnix(cell.CooldownUntil),
		cell.StateJSON,
		cell.CreatedAt.Unix(),
		cell.UpdatedAt.Unix(),
	)
	return err
}

func (s *SQLiteStore) DeleteEgressCell(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM egress_cells WHERE id = ?", id)
	return err
}
