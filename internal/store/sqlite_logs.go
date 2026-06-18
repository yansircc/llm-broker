package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/requestlog"
)

const requestLogLedgerJoinSQL = `
	LEFT JOIN billing_ledger bl
		ON bl.idempotency_key = 'usage:' || rl.request_id
		AND bl.kind = 'usage_debit'`

const requestLogEffectiveCostSQL = `COALESCE((-bl.amount_micros) / 1000000.0, rl.cost_usd)`

// InsertRequestLog persists the slim 18-column row and returns the assigned id.
// Observation payload (headers/body/meta) is no longer stored in SQL — callers
// should write the file via requestlog.WriteLogFile using the returned id.
func (s *SQLiteStore) InsertRequestLog(ctx context.Context, l *domain.RequestLog) (int64, error) {
	if l == nil {
		return 0, nil
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO request_log (
			user_id, request_id, api_key_id, account_id, provider, surface, model, cell_id,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, cost_usd,
			status, effect_kind, upstream_status, upstream_error_type,
			duration_ms, created_at
		)
		VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?
		)`,
		l.UserID, l.RequestID, l.APIKeyID, l.AccountID, l.Provider, l.Surface, l.Model, l.CellID,
		l.InputTokens, l.OutputTokens, l.CacheReadTokens, l.CacheCreateTokens, l.CostUSD,
		l.Status, l.EffectKind, l.UpstreamStatus, l.UpstreamErrorType,
		l.DurationMs, l.CreatedAt.Unix())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	l.ID = id
	return id, nil
}

func (s *SQLiteStore) QueryRequestLogs(ctx context.Context, opts domain.RequestLogQuery) ([]*domain.RequestLog, int, error) {
	where, args := buildLogWhere(opts, "rl")

	var total int
	_ = s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM request_log rl WHERE %s", where), args...).Scan(&total)

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	fetchArgs := make([]interface{}, len(args))
	copy(fetchArgs, args)
	fetchArgs = append(fetchArgs, limit, opts.Offset)

	query := fmt.Sprintf(`SELECT rl.id, rl.user_id, rl.request_id, rl.api_key_id, rl.account_id, rl.provider, rl.surface, rl.model, rl.cell_id,
		rl.input_tokens, rl.output_tokens, rl.cache_read_tokens, rl.cache_create_tokens, %s,
		rl.status, rl.effect_kind, rl.upstream_status, rl.upstream_error_type,
		rl.duration_ms, rl.created_at
		FROM request_log rl %s WHERE %s ORDER BY rl.created_at DESC LIMIT ? OFFSET ?`,
		requestLogEffectiveCostSQL, requestLogLedgerJoinSQL, where)

	rows, err := s.db.QueryContext(ctx, query, fetchArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var logs []*domain.RequestLog
	for rows.Next() {
		l := &domain.RequestLog{}
		var ts int64
		if err := rows.Scan(&l.ID, &l.UserID, &l.RequestID, &l.APIKeyID, &l.AccountID, &l.Provider, &l.Surface, &l.Model, &l.CellID,
			&l.InputTokens, &l.OutputTokens, &l.CacheReadTokens, &l.CacheCreateTokens,
			&l.CostUSD, &l.Status, &l.EffectKind, &l.UpstreamStatus, &l.UpstreamErrorType,
			&l.DurationMs, &ts); err != nil {
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

// PurgeOldLogs deletes both SQL rows and on-disk log files older than `before`.
// The file purge walks <logDir>/YYYY/MM/DD at day granularity (best-effort).
func (s *SQLiteStore) PurgeOldLogs(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM request_log WHERE created_at < ?", before.Unix())
	if err != nil {
		return 0, err
	}
	if s.logBlobDir != "" {
		requestlog.PurgeLogsBefore(s.logBlobDir, before)
	}
	return res.RowsAffected()
}

func buildLogWhere(opts domain.RequestLogQuery, alias string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	if opts.UserID != "" {
		where += " AND " + prefix + "user_id = ?"
		args = append(args, opts.UserID)
	}
	if opts.APIKeyID != "" {
		where += " AND " + prefix + "api_key_id = ?"
		args = append(args, opts.APIKeyID)
	}
	if opts.AccountID != "" {
		where += " AND " + prefix + "account_id = ?"
		args = append(args, opts.AccountID)
	}
	if opts.Model != "" {
		where += " AND " + prefix + "model = ?"
		args = append(args, opts.Model)
	}
	if opts.Since != nil {
		where += " AND " + prefix + "created_at >= ?"
		args = append(args, opts.Since.Unix())
	}
	if opts.Until != nil {
		where += " AND " + prefix + "created_at < ?"
		args = append(args, opts.Until.Unix())
	}
	if opts.FailuresOnly {
		where += " AND " + prefix + "status <> 'ok'"
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

	baseArgs := make([]interface{}, 0, len(periods)*10+3)
	selects := make([]string, 0, len(periods)*5)
	for _, p := range periods {
		selects = append(selects,
			"COUNT(CASE WHEN rl.created_at >= ? AND rl.created_at < ? THEN 1 END)",
			"COALESCE(SUM(CASE WHEN rl.created_at >= ? AND rl.created_at < ? THEN rl.input_tokens ELSE 0 END),0)",
			"COALESCE(SUM(CASE WHEN rl.created_at >= ? AND rl.created_at < ? THEN rl.output_tokens ELSE 0 END),0)",
			"COALESCE(SUM(CASE WHEN rl.created_at >= ? AND rl.created_at < ? THEN rl.cache_read_tokens ELSE 0 END),0)",
			fmt.Sprintf("COALESCE(SUM(CASE WHEN rl.created_at >= ? AND rl.created_at < ? THEN %s ELSE 0 END),0)", requestLogEffectiveCostSQL),
		)
		for i := 0; i < 5; i++ {
			baseArgs = append(baseArgs, p.since.Unix(), p.until.Unix())
		}
	}
	where := "rl.status = 'ok' AND rl.created_at >= ? AND rl.created_at < ?"
	baseArgs = append(baseArgs, periods[len(periods)-1].since.Unix(), now.Unix())
	if userID != "" {
		where += " AND rl.user_id = ?"
		baseArgs = append(baseArgs, userID)
	}

	row := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM request_log rl %s WHERE %s", strings.Join(selects, ", "), requestLogLedgerJoinSQL, where),
		baseArgs...,
	)

	scanTargets := make([]interface{}, 0, len(periods)*5)
	result := make([]domain.UsagePeriod, len(periods))
	for i, p := range periods {
		result[i].Label = p.label
		scanTargets = append(scanTargets,
			&result[i].Requests,
			&result[i].InputTokens,
			&result[i].OutputTokens,
			&result[i].CacheReadTokens,
			&result[i].CostUSD,
		)
	}
	if err := row.Scan(scanTargets...); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SQLiteStore) QueryUserTotalCosts(ctx context.Context) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT rl.user_id, COALESCE(SUM(%s),0)
		FROM request_log rl %s
		WHERE rl.status = 'ok' GROUP BY rl.user_id`, requestLogEffectiveCostSQL, requestLogLedgerJoinSQL))
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

func (s *SQLiteStore) QueryUserTotalCostsByIDs(ctx context.Context, userIDs []string) (map[string]float64, error) {
	result := make(map[string]float64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, 0, len(userIDs))
	for i, userID := range userIDs {
		placeholders[i] = "?"
		args = append(args, userID)
		result[userID] = 0
	}
	query := fmt.Sprintf(
		`SELECT rl.user_id, COALESCE(SUM(%s),0)
		FROM request_log rl %s
		WHERE rl.user_id IN (%s) AND rl.status = 'ok'
		GROUP BY rl.user_id`,
		requestLogEffectiveCostSQL, requestLogLedgerJoinSQL, strings.Join(placeholders, ","),
	)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		var cost float64
		if err := rows.Scan(&userID, &cost); err != nil {
			return nil, err
		}
		result[userID] = cost
	}
	return result, rows.Err()
}

func (s *SQLiteStore) QueryModelUsage(ctx context.Context, userID string) ([]domain.ModelUsageRow, error) {
	sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour).Unix()
	var where string
	var args []interface{}
	if userID != "" {
		where = "rl.user_id = ? AND rl.status = 'ok' AND rl.created_at >= ?"
		args = []interface{}{userID, sevenDaysAgo}
	} else {
		where = "rl.status = 'ok' AND rl.created_at >= ?"
		args = []interface{}{sevenDaysAgo}
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT rl.model, COUNT(*), COALESCE(SUM(rl.input_tokens),0), COALESCE(SUM(rl.output_tokens),0),
		COALESCE(SUM(rl.cache_read_tokens),0), COALESCE(SUM(%s),0)
		FROM request_log rl %s WHERE %s GROUP BY rl.model ORDER BY SUM(rl.input_tokens + rl.output_tokens) DESC`,
		requestLogEffectiveCostSQL, requestLogLedgerJoinSQL, where), args...)
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
