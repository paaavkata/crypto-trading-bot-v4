package kucoin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

const (
	BaseURL    = "https://api.kucoin.com"
	SandboxURL = "https://openapi-sandbox.kucoin.com"
)

type Client struct {
	client     *resty.Client
	apiKey     string
	apiSecret  string
	passphrase string
	sandbox    bool
	logger     *logrus.Logger
}

type Config struct {
	APIKey     string
	APISecret  string
	Passphrase string
	Sandbox    bool
}

func NewClient(config Config, logger *logrus.Logger) *Client {
	client := resty.New()

	baseURL := BaseURL
	if config.Sandbox {
		baseURL = SandboxURL
	}

	client.SetBaseURL(baseURL)
	client.SetTimeout(30 * time.Second)
	client.SetRetryCount(3)
	client.SetRetryWaitTime(1 * time.Second)

	return &Client{
		client:     client,
		apiKey:     config.APIKey,
		apiSecret:  config.APISecret,
		passphrase: config.Passphrase,
		sandbox:    config.Sandbox,
		logger:     logger,
	}
}

func (c *Client) generateSignature(timestamp, method, endpoint, body string) string {
	message := timestamp + method + endpoint + body
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) generatePassphraseSignature() string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(c.passphrase))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) setAuthHeaders(req *resty.Request, method, endpoint, body string) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := c.generateSignature(timestamp, method, endpoint, body)
	passphraseSignature := c.generatePassphraseSignature()

	req.SetHeaders(map[string]string{
		"KC-API-KEY":         c.apiKey,
		"KC-API-SIGN":        signature,
		"KC-API-TIMESTAMP":   timestamp,
		"KC-API-PASSPHRASE":  passphraseSignature,
		"KC-API-KEY-VERSION": "2",
		"Content-Type":       "application/json",
	})
}

func (c *Client) GetAllTickers() (*AllTickersResponse, error) {
	endpoint := "/api/v1/market/allTickers"

	req := c.client.R()

	resp, err := req.Get(endpoint)
	if err != nil {
		c.logger.WithError(err).Error("Failed to fetch all tickers")
		return nil, fmt.Errorf("failed to fetch tickers: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Code != "200000" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	// Convert data to AllTickersResponse
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var tickersResp AllTickersResponse
	if err := json.Unmarshal(dataBytes, &tickersResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tickers: %w", err)
	}

	c.logger.WithField("ticker_count", len(tickersResp.Ticker)).Info("Successfully fetched all tickers")

	return &tickersResp, nil
}

func (c *Client) GetSymbols() ([]Symbol, error) {
	endpoint := "/api/v1/symbols"

	req := c.client.R()

	resp, err := req.Get(endpoint)
	if err != nil {
		c.logger.WithError(err).Error("Failed to fetch symbols")
		return nil, fmt.Errorf("failed to fetch symbols: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Code != "200000" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var symbols []Symbol
	if err := json.Unmarshal(dataBytes, &symbols); err != nil {
		return nil, fmt.Errorf("failed to unmarshal symbols: %w", err)
	}

	c.logger.WithField("symbol_count", len(symbols)).Info("Successfully fetched symbols")

	return symbols, nil
}

func (c *Client) PlaceOrder(order OrderRequest) (*OrderResponse, error) {
	endpoint := "/api/v1/orders"

	bodyBytes, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order: %w", err)
	}

	req := c.client.R().SetBody(bodyBytes)
	c.setAuthHeaders(req, "POST", endpoint, string(bodyBytes))

	resp, err := req.Post(endpoint)
	if err != nil {
		c.logger.WithError(err).Error("Failed to place order")
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Code != "200000" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(dataBytes, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"order_id": orderResp.OrderId,
		"symbol":   order.Symbol,
		"side":     order.Side,
	}).Info("Order placed successfully")

	return &orderResp, nil
}
