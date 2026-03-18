package store

import (
	"context"
	"fmt"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) InsertRequestLog(ctx context.Context, l *domain.RequestLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO request_log (
			user_id, account_id, provider, surface, model, path, cell_id, bucket_key,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, cost_usd,
			status, effect_kind, upstream_status, upstream_request_id, request_bytes, attempt_count,
			duration_ms, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.UserID, l.AccountID, l.Provider, l.Surface, l.Model, l.Path, l.CellID, l.BucketKey,
		l.InputTokens, l.OutputTokens, l.CacheReadTokens, l.CacheCreateTokens, l.CostUSD,
		l.Status, l.EffectKind, l.UpstreamStatus, l.UpstreamRequestID, l.RequestBytes, l.AttemptCount,
		l.DurationMs, l.CreatedAt.Unix())
	return err
}

func (s *SQLiteStore) QueryRequestLogs(ctx context.Context, opts domain.RequestLogQuery) ([]*domain.RequestLog, int, error) {
	where, args := buildLogWhere(opts.UserID, opts.AccountID, opts.FailuresOnly)

	var total int
	_ = s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM request_log WHERE %s", where), args...).Scan(&total)

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	fetchArgs := make([]interface{}, len(args))
	copy(fetchArgs, args)
	fetchArgs = append(fetchArgs, limit, opts.Offset)

	query := fmt.Sprintf(`SELECT id, user_id, account_id, provider, surface, model, path, cell_id, bucket_key,
		input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, cost_usd,
		status, effect_kind, upstream_status, upstream_request_id, request_bytes, attempt_count,
		duration_ms, created_at
		FROM request_log WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, where)

	rows, err := s.db.QueryContext(ctx, query, fetchArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var logs []*domain.RequestLog
	for rows.Next() {
		l := &domain.RequestLog{}
		var ts int64
		if err := rows.Scan(&l.ID, &l.UserID, &l.AccountID, &l.Provider, &l.Surface, &l.Model, &l.Path, &l.CellID, &l.BucketKey,
			&l.InputTokens, &l.OutputTokens, &l.CacheReadTokens, &l.CacheCreateTokens,
			&l.CostUSD, &l.Status, &l.EffectKind, &l.UpstreamStatus, &l.UpstreamRequestID,
			&l.RequestBytes, &l.AttemptCount, &l.DurationMs, &ts); err != nil {
			return nil, 0, err
		}
		l.CreatedAt = time.Unix(ts, 0).UTC()
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (s *SQLiteStore) QueryRelayOutcomeStats(ctx context.Context, since time.Time) ([]domain.RelayOutcomeStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider, surface, effect_kind, upstream_status,
			COUNT(*),
			COUNT(DISTINCT user_id),
			COUNT(DISTINCT account_id),
			MAX(created_at)
		FROM request_log
		WHERE created_at >= ?
		GROUP BY provider, surface, effect_kind, upstream_status
		ORDER BY COUNT(*) DESC, provider, surface, effect_kind, upstream_status
	`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.RelayOutcomeStat
	for rows.Next() {
		var stat domain.RelayOutcomeStat
		var lastSeen int64
		if err := rows.Scan(
			&stat.Provider,
			&stat.Surface,
			&stat.EffectKind,
			&stat.UpstreamStatus,
			&stat.Requests,
			&stat.DistinctUsers,
			&stat.DistinctAccounts,
			&lastSeen,
		); err != nil {
			return nil, err
		}
		stat.LastSeenAt = time.Unix(lastSeen, 0).UTC()
		result = append(result, stat)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) QueryCellRiskStats(ctx context.Context, since time.Time) ([]domain.CellRiskStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(cell_id, ''), provider,
			COUNT(*),
			SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END),
			SUM(CASE WHEN upstream_status = 400 THEN 1 ELSE 0 END),
			SUM(CASE WHEN upstream_status = 403 THEN 1 ELSE 0 END),
			SUM(CASE WHEN upstream_status = 429 THEN 1 ELSE 0 END),
			SUM(CASE WHEN effect_kind = 'block' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'transport_error' THEN 1 ELSE 0 END),
			COUNT(DISTINCT user_id),
			COUNT(DISTINCT account_id),
			MAX(created_at)
		FROM request_log
		WHERE created_at >= ?
		GROUP BY COALESCE(cell_id, ''), provider
		ORDER BY
			(SUM(CASE WHEN upstream_status IN (400, 403, 429) THEN 1 ELSE 0 END) +
			 SUM(CASE WHEN effect_kind = 'block' THEN 1 ELSE 0 END) +
			 SUM(CASE WHEN status = 'transport_error' THEN 1 ELSE 0 END)) DESC,
			COUNT(*) DESC,
			provider,
			COALESCE(cell_id, '')
	`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.CellRiskStat
	for rows.Next() {
		var stat domain.CellRiskStat
		var lastSeen int64
		if err := rows.Scan(
			&stat.CellID,
			&stat.Provider,
			&stat.Requests,
			&stat.Successes,
			&stat.Status400,
			&stat.Status403,
			&stat.Status429,
			&stat.Blocks,
			&stat.TransportErrors,
			&stat.DistinctUsers,
			&stat.DistinctAccounts,
			&lastSeen,
		); err != nil {
			return nil, err
		}
		stat.LastSeenAt = time.Unix(lastSeen, 0).UTC()
		result = append(result, stat)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) PurgeOldLogs(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM request_log WHERE created_at < ?", before.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func buildLogWhere(userID, accountID string, failuresOnly bool) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if userID != "" {
		where += " AND user_id = ?"
		args = append(args, userID)
	}
	if accountID != "" {
		where += " AND account_id = ?"
		args = append(args, accountID)
	}
	if failuresOnly {
		where += " AND status <> 'ok'"
	}
	return where, args
}

func (s *SQLiteStore) QueryUsagePeriods(ctx context.Context, userID string, loc *time.Location) ([]domain.UsagePeriod, error) {
	if loc == nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterdayStart := todayStart.Add(-24 * time.Hour)

	periods := []struct {
		label string
		since time.Time
		until time.Time
	}{
		{"today", todayStart, now},
		{"yesterday", yesterdayStart, todayStart},
		{"3 days", now.Add(-3 * 24 * time.Hour), now},
		{"7 days", now.Add(-7 * 24 * time.Hour), now},
		{"30 days", now.Add(-30 * 24 * time.Hour), now},
	}

	result := make([]domain.UsagePeriod, 0, len(periods))
	for _, p := range periods {
		var where string
		var args []interface{}
		if userID != "" {
			where = "user_id = ? AND status = 'ok' AND created_at >= ? AND created_at < ?"
			args = []interface{}{userID, p.since.Unix(), p.until.Unix()}
		} else {
			where = "status = 'ok' AND created_at >= ? AND created_at < ?"
			args = []interface{}{p.since.Unix(), p.until.Unix()}
		}
		row := s.db.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
			COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cost_usd),0)
			FROM request_log WHERE %s`, where), args...)
		up := domain.UsagePeriod{Label: p.label}
		row.Scan(&up.Requests, &up.InputTokens, &up.OutputTokens, &up.CacheReadTokens, &up.CostUSD)
		result = append(result, up)
	}
	return result, nil
}

func (s *SQLiteStore) QueryUserTotalCosts(ctx context.Context) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, COALESCE(SUM(cost_usd),0) FROM request_log WHERE status = 'ok' GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]float64)
	for rows.Next() {
		var userID string
		var cost float64
		rows.Scan(&userID, &cost)
		result[userID] = cost
	}
	return result, rows.Err()
}

func (s *SQLiteStore) QueryModelUsage(ctx context.Context, userID string) ([]domain.ModelUsageRow, error) {
	sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour).Unix()
	var where string
	var args []interface{}
	if userID != "" {
		where = "user_id = ? AND status = 'ok' AND created_at >= ?"
		args = []interface{}{userID, sevenDaysAgo}
	} else {
		where = "status = 'ok' AND created_at >= ?"
		args = []interface{}{sevenDaysAgo}
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT model, COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM request_log WHERE %s GROUP BY model ORDER BY SUM(input_tokens + output_tokens) DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.ModelUsageRow
	for rows.Next() {
		var m domain.ModelUsageRow
		rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.CacheReadTokens, &m.CostUSD)
		result = append(result, m)
	}
	return result, rows.Err()
}
