package zpay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultMAPIURL = "https://zpayz.cn/mapi.php"
	defaultAPIURL  = "https://zpayz.cn/api.php"
)

type Config struct {
	PID        string
	Key        string
	MAPIURL    string
	APIURL     string
	HTTPClient *http.Client
}

type Client struct {
	pid        string
	key        string
	mapiURL    string
	apiURL     string
	httpClient *http.Client
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	mapiURL := cfg.MAPIURL
	if mapiURL == "" {
		mapiURL = defaultMAPIURL
	}
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	return &Client{
		pid:        cfg.PID,
		key:        cfg.Key,
		mapiURL:    mapiURL,
		apiURL:     apiURL,
		httpClient: httpClient,
	}
}

type CreateQRCodeOrderRequest struct {
	Type       string
	Name       string
	Money      string
	OutTradeNo string
	NotifyURL  string
	ClientIP   string
	Device     string
	Param      string
	CID        string
}

type CreateQRCodeOrderResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	QRCode  string `json:"qrcode"`
	Image   string `json:"img"`
	PayURL  string `json:"payurl"`
	TradeNo string `json:"trade_no"`
}

func (c *Client) CreateQRCodeOrder(ctx context.Context, in CreateQRCodeOrderRequest) (CreateQRCodeOrderResponse, error) {
	params := map[string]string{
		"pid":          c.pid,
		"type":         in.Type,
		"name":         in.Name,
		"money":        in.Money,
		"out_trade_no": in.OutTradeNo,
		"notify_url":   in.NotifyURL,
		"clientip":     in.ClientIP,
		"device":       in.Device,
		"param":        in.Param,
		"cid":          in.CID,
		"sign_type":    "MD5",
	}
	params["sign"] = Sign(params, c.key)

	form := make(url.Values, len(params))
	for k, v := range params {
		if v == "" {
			continue
		}
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.mapiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return CreateQRCodeOrderResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out CreateQRCodeOrderResponse
	if err := c.doJSON(req, &out); err != nil {
		return CreateQRCodeOrderResponse{}, err
	}
	return out, nil
}

type QueryOrderRequest struct {
	OutTradeNo string
}

type QueryOrderResponse struct {
	Code    int    `json:"code"`
	Status  int    `json:"status"`
	Type    string `json:"type"`
	Money   string `json:"money"`
	TradeNo string `json:"trade_no"`
}

func (c *Client) QueryOrder(ctx context.Context, in QueryOrderRequest) (QueryOrderResponse, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return QueryOrderResponse{}, err
	}
	q := u.Query()
	q.Set("act", "order")
	q.Set("pid", c.pid)
	q.Set("key", c.key)
	q.Set("out_trade_no", in.OutTradeNo)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return QueryOrderResponse{}, err
	}

	var out QueryOrderResponse
	if err := c.doJSON(req, &out); err != nil {
		return QueryOrderResponse{}, err
	}
	return out, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("zpay %s: %s", req.URL.Path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
