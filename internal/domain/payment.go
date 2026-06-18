package domain

import "time"

type PaymentOrder struct {
	ID                 string     `json:"id"`
	OutTradeNo         string     `json:"out_trade_no"`
	UserID             string     `json:"user_id"`
	Gateway            string     `json:"gateway"`
	Status             string     `json:"status"`
	ProductName        string     `json:"product_name"`
	AmountCNYFen       int64      `json:"amount_cny_fen"`
	CreditMicros       int64      `json:"credit_micros"`
	ExchangeRateMicros int64      `json:"exchange_rate_micros"`
	PaymentType        string     `json:"payment_type"`
	ZpayTradeNo        string     `json:"zpay_trade_no"`
	QRCode             string     `json:"qrcode"`
	QRImage            string     `json:"qr_image"`
	CreatedAt          time.Time  `json:"created_at"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
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
