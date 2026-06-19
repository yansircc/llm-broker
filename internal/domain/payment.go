package domain

import "time"

type PaymentOrder struct {
	ID                   string     `json:"id"`
	OutTradeNo           string     `json:"out_trade_no"`
	UserID               string     `json:"user_id"`
	Gateway              string     `json:"gateway"`
	IntegrationID        string     `json:"integration_id"`
	Status               string     `json:"status"`
	ProductName          string     `json:"product_name"`
	AmountCNYFen         int64      `json:"amount_cny_fen"`
	CreditMicros         int64      `json:"credit_micros"`
	ExchangeRateMicros   int64      `json:"exchange_rate_micros"`
	PaymentType          string     `json:"payment_type"`
	ZpayTradeNo          string     `json:"zpay_trade_no"`
	QRCode               string     `json:"qrcode"`
	QRImage              string     `json:"qr_image"`
	ProviderOrderID      string     `json:"provider_order_id"`
	ProviderPaymentID    string     `json:"provider_payment_id"`
	Method               string     `json:"method"`
	SettlementCurrency   string     `json:"settlement_currency"`
	AmountMinor          int64      `json:"amount_minor"`
	CheckoutURL          string     `json:"checkout_url"`
	ProviderMetadataJSON string     `json:"provider_metadata_json"`
	CreatedAt            time.Time  `json:"created_at"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type PaymentEvent struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"order_id"`
	Gateway        string    `json:"gateway"`
	EventType      string    `json:"event_type"`
	ValidSignature bool      `json:"valid_signature"`
	PayloadJSON    string    `json:"payload_json"`
	CreatedAt      time.Time `json:"created_at"`
}

type PaymentOrderSummary struct {
	TotalOrders         int   `json:"total_orders"`
	PendingOrders       int   `json:"pending_orders"`
	PaidOrders          int   `json:"paid_orders"`
	PaidAmountCNYFen    int64 `json:"paid_amount_cny_fen"`
	PaidCreditMicros    int64 `json:"paid_credit_micros"`
	PendingCreditMicros int64 `json:"pending_credit_micros"`
}
