package store

import (
	"context"
	"fmt"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) SaveSecurityEvent(ctx context.Context, event *domain.SecurityEvent) error {
	if event == nil {
		return nil
	}
	success := 0
	if event.Success {
		success = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO security_events (id, kind, ip_hash, email_hash, success, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.Kind, event.IPHash, event.EmailHash, success, event.Reason, event.CreatedAt.UTC().Unix())
	return err
}

func (s *SQLiteStore) CountSecurityEvents(ctx context.Context, q domain.SecurityEventQuery) (int, error) {
	where := "kind = ? AND created_at >= ?"
	args := []any{q.Kind, q.Since.UTC().Unix()}
	if q.IPHash != "" {
		where += " AND ip_hash = ?"
		args = append(args, q.IPHash)
	}
	if q.EmailHash != "" {
		where += " AND email_hash = ?"
		args = append(args, q.EmailHash)
	}
	if q.Success != nil {
		where += " AND success = ?"
		if *q.Success {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	var count int
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM security_events WHERE %s", where), args...).Scan(&count)
	return count, err
}
