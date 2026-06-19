package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) UpsertRuntimeSetting(ctx context.Context, setting *domain.RuntimeSetting) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runtime_settings (key, value_json, updated_at, updated_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value_json = excluded.value_json,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
	`, setting.Key, setting.ValueJSON, setting.UpdatedAt.Unix(), setting.UpdatedBy)
	return err
}

func (s *SQLiteStore) GetRuntimeSetting(ctx context.Context, key string) (*domain.RuntimeSetting, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT key, value_json, updated_at, updated_by
		FROM runtime_settings WHERE key = ?
	`, key)
	return scanRuntimeSetting(row)
}

func (s *SQLiteStore) ListRuntimeSettings(ctx context.Context) ([]*domain.RuntimeSetting, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key, value_json, updated_at, updated_by
		FROM runtime_settings ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.RuntimeSetting
	for rows.Next() {
		setting, err := scanRuntimeSetting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, setting)
	}
	return out, rows.Err()
}

func scanRuntimeSetting(scanner interface{ Scan(...any) error }) (*domain.RuntimeSetting, error) {
	var setting domain.RuntimeSetting
	var updatedAt int64
	err := scanner.Scan(&setting.Key, &setting.ValueJSON, &updatedAt, &setting.UpdatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	setting.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &setting, nil
}

func (s *SQLiteStore) SaveIntegration(ctx context.Context, integration *domain.Integration) error {
	enabled := 0
	if integration.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO integrations (
			id, kind, provider, display_name, enabled, priority, config_json,
			secret_json_enc, secret_fingerprint, created_at, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind = excluded.kind,
			provider = excluded.provider,
			display_name = excluded.display_name,
			enabled = excluded.enabled,
			priority = excluded.priority,
			config_json = excluded.config_json,
			secret_json_enc = excluded.secret_json_enc,
			secret_fingerprint = excluded.secret_fingerprint,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
	`,
		integration.ID, integration.Kind, integration.Provider, integration.DisplayName,
		enabled, integration.Priority, integration.ConfigJSON, integration.SecretJSONEnc,
		integration.SecretFingerprint, integration.CreatedAt.Unix(), integration.UpdatedAt.Unix(),
		integration.UpdatedBy)
	return err
}

func (s *SQLiteStore) GetIntegration(ctx context.Context, id string) (*domain.Integration, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, kind, provider, display_name, enabled, priority, config_json,
			secret_json_enc, secret_fingerprint, created_at, updated_at, updated_by
		FROM integrations WHERE id = ?
	`, id)
	return scanIntegration(row)
}

func (s *SQLiteStore) ListIntegrations(ctx context.Context, kind string) ([]*domain.Integration, error) {
	q := `
		SELECT id, kind, provider, display_name, enabled, priority, config_json,
			secret_json_enc, secret_fingerprint, created_at, updated_at, updated_by
		FROM integrations
	`
	args := []any{}
	if kind != "" {
		q += " WHERE kind = ?"
		args = append(args, kind)
	}
	q += " ORDER BY kind, priority, provider, display_name"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Integration
	for rows.Next() {
		integration, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, integration)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListEnabledIntegrations(ctx context.Context, kind, provider string) ([]*domain.Integration, error) {
	q := `
		SELECT id, kind, provider, display_name, enabled, priority, config_json,
			secret_json_enc, secret_fingerprint, created_at, updated_at, updated_by
		FROM integrations
		WHERE kind = ? AND enabled = 1
	`
	args := []any{kind}
	if provider != "" {
		q += " AND provider = ?"
		args = append(args, provider)
	}
	q += " ORDER BY priority, provider, display_name"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Integration
	for rows.Next() {
		integration, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, integration)
	}
	return out, rows.Err()
}

func scanIntegration(scanner interface{ Scan(...any) error }) (*domain.Integration, error) {
	var integration domain.Integration
	var enabled int
	var createdAt, updatedAt int64
	err := scanner.Scan(
		&integration.ID, &integration.Kind, &integration.Provider, &integration.DisplayName,
		&enabled, &integration.Priority, &integration.ConfigJSON, &integration.SecretJSONEnc,
		&integration.SecretFingerprint, &createdAt, &updatedAt, &integration.UpdatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	integration.Enabled = enabled != 0
	integration.CreatedAt = time.Unix(createdAt, 0).UTC()
	integration.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &integration, nil
}

func (s *SQLiteStore) SaveIntegrationEvent(ctx context.Context, event *domain.IntegrationEvent) error {
	success := 0
	if event.Success {
		success = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO integration_events (
			id, integration_id, kind, event_type, success, error_code, redacted_payload_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.IntegrationID, event.Kind, event.EventType, success, event.ErrorCode, event.RedactedPayloadJSON, event.CreatedAt.Unix())
	return err
}

func (s *SQLiteStore) SaveSettingsAudit(ctx context.Context, audit *domain.SettingsAudit) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings_audit (
			id, actor_user_id, target_type, target_id, action, before_json, after_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, audit.ID, audit.ActorUserID, audit.TargetType, audit.TargetID, audit.Action, audit.BeforeJSON, audit.AfterJSON, audit.CreatedAt.Unix())
	return err
}
