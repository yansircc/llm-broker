package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/payments/zpay"
)

type Config struct {
	Public  map[string]string
	Secrets map[string]string
}

type CreateOrderRequest struct {
	Method      string
	Name        string
	AmountMinor int64
	OutTradeNo  string
	NotifyURL   string
	ClientIP    string
	UserID      string
}

type CreatedOrder struct {
	ProviderOrderID   string
	ProviderPaymentID string
	Method            string
	CheckoutURL       string
	QRCode            string
	QRImage           string
	MetadataJSON      string
}

type ProviderEvent struct {
	OutTradeNo        string
	ProviderPaymentID string
	Method            string
	Status            string
	AmountMinor       int64
	Currency          string
	PayloadJSON       string
}

type Provider interface {
	Name() string
	CreateOrder(ctx context.Context, cfg Config, req CreateOrderRequest) (CreatedOrder, error)
	VerifyWebhook(ctx context.Context, cfg Config, r *http.Request, order *domain.PaymentOrder) (ProviderEvent, bool, error)
	QueryOrder(ctx context.Context, cfg Config, order *domain.PaymentOrder) (ProviderEvent, error)
}

func ProviderByName(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "zpay", "7pay":
		return ZPayProvider{}
	default:
		return nil
	}
}

type ZPayProvider struct{}

func (ZPayProvider) Name() string { return "zpay" }

func (ZPayProvider) CreateOrder(ctx context.Context, cfg Config, req CreateOrderRequest) (CreatedOrder, error) {
	client, cid, err := zpayClient(cfg)
	if err != nil {
		return CreatedOrder{}, err
	}
	method := req.Method
	if method == "" {
		method = "alipay"
	}
	resp, err := client.CreateQRCodeOrder(ctx, zpay.CreateQRCodeOrderRequest{
		Type:       method,
		Name:       req.Name,
		Money:      fenToYuan(req.AmountMinor),
		OutTradeNo: req.OutTradeNo,
		NotifyURL:  req.NotifyURL,
		ClientIP:   req.ClientIP,
		Device:     "pc",
		Param:      req.UserID,
		CID:        cid,
	})
	if err != nil {
		return CreatedOrder{}, err
	}
	if resp.Code != 1 {
		if resp.Msg == "" {
			resp.Msg = "zpay create order failed"
		}
		return CreatedOrder{}, errors.New(resp.Msg)
	}
	metadata, _ := json.Marshal(map[string]string{"pay_url": resp.PayURL})
	return CreatedOrder{
		ProviderOrderID:   resp.TradeNo,
		ProviderPaymentID: resp.TradeNo,
		Method:            method,
		CheckoutURL:       resp.PayURL,
		QRCode:            resp.QRCode,
		QRImage:           resp.Image,
		MetadataJSON:      string(metadata),
	}, nil
}

func (ZPayProvider) VerifyWebhook(_ context.Context, cfg Config, r *http.Request, _ *domain.PaymentOrder) (ProviderEvent, bool, error) {
	params, err := paymentParams(r)
	if err != nil {
		return ProviderEvent{}, false, err
	}
	payload, _ := json.Marshal(params)
	key := strings.TrimSpace(cfg.Secrets["key"])
	valid := key != "" && zpay.Verify(params, key)
	amount, _ := parseYuanToFen(params["money"])
	return ProviderEvent{
		OutTradeNo:        params["out_trade_no"],
		ProviderPaymentID: params["trade_no"],
		Method:            params["type"],
		Status:            params["trade_status"],
		AmountMinor:       amount,
		Currency:          "CNY",
		PayloadJSON:       string(payload),
	}, valid, nil
}

func (ZPayProvider) QueryOrder(ctx context.Context, cfg Config, order *domain.PaymentOrder) (ProviderEvent, error) {
	client, _, err := zpayClient(cfg)
	if err != nil {
		return ProviderEvent{}, err
	}
	resp, err := client.QueryOrder(ctx, zpay.QueryOrderRequest{OutTradeNo: order.OutTradeNo})
	if err != nil {
		return ProviderEvent{}, err
	}
	payload, _ := json.Marshal(resp)
	amount, _ := parseYuanToFen(resp.Money)
	status := "pending"
	if resp.Code == 1 && resp.Status == 1 {
		status = "TRADE_SUCCESS"
	}
	return ProviderEvent{
		OutTradeNo:        order.OutTradeNo,
		ProviderPaymentID: resp.TradeNo,
		Method:            resp.Type,
		Status:            status,
		AmountMinor:       amount,
		Currency:          "CNY",
		PayloadJSON:       string(payload),
	}, nil
}

func zpayClient(cfg Config) (*zpay.Client, string, error) {
	pid := strings.TrimSpace(cfg.Public["pid"])
	key := strings.TrimSpace(cfg.Secrets["key"])
	if pid == "" || key == "" {
		return nil, "", fmt.Errorf("zpay is not configured")
	}
	return zpay.NewClient(zpay.Config{PID: pid, Key: key, HTTPClient: http.DefaultClient}), strings.TrimSpace(cfg.Public["cid"]), nil
}

func paymentParams(r *http.Request) (map[string]string, error) {
	values := r.URL.Query()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		values = r.Form
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	if out["sign"] == "" || out["out_trade_no"] == "" {
		return nil, fmt.Errorf("missing required params")
	}
	return out, nil
}

func fenToYuan(fen int64) string {
	return fmt.Sprintf("%.2f", float64(fen)/100)
}

func parseYuanToFen(raw string) (int64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}
