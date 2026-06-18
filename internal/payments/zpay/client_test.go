package zpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCreateQRCodeOrderPostsSignedForm(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Fatalf("content-type = %q, want application/x-www-form-urlencoded", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotForm = r.PostForm
		_ = json.NewEncoder(w).Encode(CreateQRCodeOrderResponse{
			Code:    1,
			Msg:     "ok",
			QRCode:  "qr",
			Image:   "img",
			PayURL:  "pay",
			TradeNo: "zpay-1",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		PID:        "pid-1",
		Key:        "secret",
		MAPIURL:    server.URL,
		APIURL:     "http://unused.test/api.php",
		HTTPClient: server.Client(),
	})
	resp, err := client.CreateQRCodeOrder(context.Background(), CreateQRCodeOrderRequest{
		Type:       "alipay",
		Name:       "Pro",
		Money:      "199.00",
		OutTradeNo: "order-1",
		NotifyURL:  "https://example.test/notify",
		ClientIP:   "1.2.3.4",
		Device:     "pc",
		Param:      "user|tier",
		CID:        "cid-1",
	})
	if err != nil {
		t.Fatalf("CreateQRCodeOrder: %v", err)
	}
	if resp.TradeNo != "zpay-1" {
		t.Fatalf("TradeNo = %q, want zpay-1", resp.TradeNo)
	}

	wantFields := map[string]string{
		"pid":          "pid-1",
		"type":         "alipay",
		"name":         "Pro",
		"money":        "199.00",
		"out_trade_no": "order-1",
		"notify_url":   "https://example.test/notify",
		"clientip":     "1.2.3.4",
		"device":       "pc",
		"param":        "user|tier",
		"cid":          "cid-1",
		"sign_type":    "MD5",
	}
	for key, want := range wantFields {
		if got := gotForm.Get(key); got != want {
			t.Fatalf("form[%s] = %q, want %q", key, got, want)
		}
	}
	if gotForm.Get("sign") != Sign(flatten(gotForm), "secret") {
		t.Fatalf("form sign = %q, want signature for submitted fields", gotForm.Get("sign"))
	}
}

func TestQueryOrderBuildsExpectedURL(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(QueryOrderResponse{
			Code:    1,
			Status:  1,
			Type:    "alipay",
			Money:   "199.00",
			TradeNo: "zpay-1",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		PID:        "pid-1",
		Key:        "secret",
		MAPIURL:    "http://unused.test/mapi.php",
		APIURL:     server.URL,
		HTTPClient: server.Client(),
	})
	resp, err := client.QueryOrder(context.Background(), QueryOrderRequest{OutTradeNo: "order-1"})
	if err != nil {
		t.Fatalf("QueryOrder: %v", err)
	}
	if resp.Status != 1 || resp.TradeNo != "zpay-1" {
		t.Fatalf("response = %#v", resp)
	}

	want := map[string]string{
		"act":          "order",
		"pid":          "pid-1",
		"key":          "secret",
		"out_trade_no": "order-1",
	}
	for key, value := range want {
		if got := gotQuery.Get(key); got != value {
			t.Fatalf("query[%s] = %q, want %q", key, got, value)
		}
	}
}

func flatten(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for key := range values {
		out[key] = values.Get(key)
	}
	return out
}
