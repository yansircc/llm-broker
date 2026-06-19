package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) SavePaymentOrder(ctx context.Context, order *domain.PaymentOrder) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payment_orders (
			id, out_trade_no, user_id, gateway, status, product_name,
			amount_cny_fen, credit_micros, exchange_rate_micros, payment_type,
			zpay_trade_no, qrcode, qr_image, created_at, paid_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(out_trade_no) DO UPDATE SET
			status = excluded.status,
			zpay_trade_no = excluded.zpay_trade_no,
			qrcode = excluded.qrcode,
			qr_image = excluded.qr_image,
			paid_at = excluded.paid_at,
			updated_at = excluded.updated_at
	`,
		order.ID, order.OutTradeNo, order.UserID, order.Gateway, order.Status, order.ProductName,
		order.AmountCNYFen, order.CreditMicros, order.ExchangeRateMicros, order.PaymentType,
		order.ZpayTradeNo, order.QRCode, order.QRImage, order.CreatedAt.Unix(), nullableUnix(order.PaidAt), order.UpdatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetPaymentOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.PaymentOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, out_trade_no, user_id, gateway, status, product_name,
			amount_cny_fen, credit_micros, exchange_rate_micros, payment_type,
			zpay_trade_no, qrcode, qr_image, created_at, paid_at, updated_at
		FROM payment_orders WHERE out_trade_no = ?
	`, outTradeNo)
	return scanPaymentOrder(row)
}

func (s *SQLiteStore) ListPaymentOrdersByUser(ctx context.Context, userID string, limit int) ([]*domain.PaymentOrder, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, out_trade_no, user_id, gateway, status, product_name,
			amount_cny_fen, credit_micros, exchange_rate_micros, payment_type,
			zpay_trade_no, qrcode, qr_image, created_at, paid_at, updated_at
		FROM payment_orders WHERE user_id = ?
		ORDER BY created_at DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []*domain.PaymentOrder
	for rows.Next() {
		order, err := scanPaymentOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *SQLiteStore) ListPaymentOrders(ctx context.Context, limit int) ([]*domain.PaymentOrder, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, out_trade_no, user_id, gateway, status, product_name,
			amount_cny_fen, credit_micros, exchange_rate_micros, payment_type,
			zpay_trade_no, qrcode, qr_image, created_at, paid_at, updated_at
		FROM payment_orders
		ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []*domain.PaymentOrder
	for rows.Next() {
		order, err := scanPaymentOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *SQLiteStore) SummarizePaymentOrders(ctx context.Context) (*domain.PaymentOrderSummary, error) {
	var summary domain.PaymentOrderSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN amount_cny_fen ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN credit_micros ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN credit_micros ELSE 0 END), 0)
		FROM payment_orders
	`).Scan(
		&summary.TotalOrders,
		&summary.PendingOrders,
		&summary.PaidOrders,
		&summary.PaidAmountCNYFen,
		&summary.PaidCreditMicros,
		&summary.PendingCreditMicros,
	)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func (s *SQLiteStore) MarkPaymentOrderPaid(ctx context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE payment_orders
		SET status = 'paid', zpay_trade_no = ?, payment_type = ?, paid_at = ?, updated_at = ?
		WHERE out_trade_no = ? AND status <> 'paid'
	`, zpayTradeNo, paymentType, paidAt.Unix(), paidAt.Unix(), outTradeNo)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		order, err := s.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
		if err != nil {
			return err
		}
		if order == nil {
			return ErrNotFound
		}
	}
	return nil
}

func (s *SQLiteStore) FulfillPaymentOrderWithCredit(ctx context.Context, outTradeNo, zpayTradeNo, paymentType string, paidAt time.Time, credit *domain.BillingLedgerEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if credit != nil && credit.AmountMicros != 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO billing_ledger (
				id, user_id, amount_micros, kind, source_type, source_id, idempotency_key,
				description, price_snapshot_json, metadata_json, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			credit.ID, credit.UserID, credit.AmountMicros, credit.Kind, credit.SourceType, credit.SourceID,
			credit.IdempotencyKey, credit.Description, credit.PriceSnapshotJSON, credit.MetadataJSON, credit.CreatedAt.Unix()); err != nil {
			return err
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE payment_orders
		SET status = 'paid', zpay_trade_no = ?, payment_type = ?, paid_at = ?, updated_at = ?
		WHERE out_trade_no = ? AND status <> 'paid'
	`, zpayTradeNo, paymentType, paidAt.Unix(), paidAt.Unix(), outTradeNo)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var id string
		err := tx.QueryRowContext(ctx, "SELECT id FROM payment_orders WHERE out_trade_no = ?", outTradeNo).Scan(&id)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) SavePaymentEvent(ctx context.Context, event *domain.PaymentEvent) error {
	valid := 0
	if event.ValidSignature {
		valid = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payment_events (id, order_id, gateway, event_type, valid_signature, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.OrderID, event.Gateway, event.EventType, valid, event.PayloadJSON, event.CreatedAt.Unix())
	return err
}

func scanPaymentOrder(scanner interface{ Scan(...any) error }) (*domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	var createdAt, updatedAt int64
	var paidAt sql.NullInt64
	err := scanner.Scan(
		&order.ID, &order.OutTradeNo, &order.UserID, &order.Gateway, &order.Status, &order.ProductName,
		&order.AmountCNYFen, &order.CreditMicros, &order.ExchangeRateMicros, &order.PaymentType,
		&order.ZpayTradeNo, &order.QRCode, &order.QRImage, &createdAt, &paidAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	order.CreatedAt = time.Unix(createdAt, 0).UTC()
	order.PaidAt = scanNullableTime(paidAt)
	order.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &order, nil
}
