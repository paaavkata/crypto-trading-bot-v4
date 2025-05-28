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

func (c *Client) GetOrder(ctx context.Context, orderID string) (*KucoinOrderDetail, error) {
	endpoint := fmt.Sprintf("/api/v1/orders/%s", orderID)

	req := c.client.R().SetContext(ctx)
	// GetOrder is a private endpoint and requires authentication
	c.setAuthHeaders(req, "GET", endpoint, "")

	resp, err := req.Get(endpoint)
	if err != nil {
		c.logger.WithError(err).WithField("order_id", orderID).Error("Failed to fetch order details")
		return nil, fmt.Errorf("failed to fetch order details for order %s: %w", orderID, err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response for order %s: %w", orderID, err)
	}

	if apiResp.Code != "200000" {
		// Specific check for order not exist
		if apiResp.Code == "400100" && apiResp.Msg == "order_not_exist" {
			c.logger.WithField("order_id", orderID).Warn("Order not found on Kucoin")
			return nil, fmt.Errorf("order %s not found on Kucoin: %w (API Code: %s, Msg: %s)", orderID, err, apiResp.Code, apiResp.Msg)
		}
		return nil, fmt.Errorf("API error for order %s: Code %s, Msg: %s", orderID, apiResp.Code, apiResp.Msg)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for order %s: %w", orderID, err)
	}

	var orderDetail KucoinOrderDetail
	if err := json.Unmarshal(dataBytes, &orderDetail); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order detail for order %s: %w", orderID, err)
	}

	// Kucoin's /api/v1/orders/{orderId} endpoint returns isActive.
	// We can infer a more conventional 'status' if needed, or the caller can use isActive.
	// For example:
	if !orderDetail.IsActive && orderDetail.DealSize == orderDetail.Size {
		orderDetail.Status = "filled" // Or "done"
	} else if !orderDetail.IsActive && orderDetail.CancelExist { // Assuming CancelExist becomes false if fully filled before any cancel action
		orderDetail.Status = "canceled"
	} else if orderDetail.IsActive {
		orderDetail.Status = "active" // Or "open", "pending"
	} else {
		// Could be partially filled and then canceled, or other edge cases.
		// Rely on isActive and filled amounts primarily if direct status field is missing/ambiguous.
		orderDetail.Status = "unknown" // Default if logic cannot determine
	}


	c.logger.WithFields(logrus.Fields{
		"order_id":   orderID,
		"symbol":     orderDetail.Symbol,
		"side":       orderDetail.Side,
		"deal_size":  orderDetail.DealSize,
		"deal_funds": orderDetail.DealFunds,
		"is_active":  orderDetail.IsActive,
		"inferred_status": orderDetail.Status,
	}).Debug("Successfully fetched order details")

	return &orderDetail, nil
}
