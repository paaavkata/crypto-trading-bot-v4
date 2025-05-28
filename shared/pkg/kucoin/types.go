package kucoin

import (
	"github.com/shopspring/decimal"
)

// Use decimal.Decimal for all monetary fields

type APIResponse struct {
	Code string      `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type Ticker struct {
	Symbol       string          `json:"symbol"`
	SymbolName   string          `json:"symbolName"`
	Buy          decimal.Decimal `json:"buy"`
	Sell         decimal.Decimal `json:"sell"`
	ChangeRate   decimal.Decimal `json:"changeRate"`
	ChangePrice  decimal.Decimal `json:"changePrice"`
	High         decimal.Decimal `json:"high"`
	Low          decimal.Decimal `json:"low"`
	Vol          decimal.Decimal `json:"vol"`
	VolValue     decimal.Decimal `json:"volValue"`
	Last         decimal.Decimal `json:"last"`
	AveragePrice decimal.Decimal `json:"averagePrice"`
}

type AllTickersResponse struct {
	Time   int64    `json:"time"`
	Ticker []Ticker `json:"ticker"`
}

type Symbol struct {
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	BaseCurrency   string `json:"baseCurrency"`
	QuoteCurrency  string `json:"quoteCurrency"`
	BaseMinSize    string `json:"baseMinSize"`
	QuoteMinSize   string `json:"quoteMinSize"`
	BaseMaxSize    string `json:"baseMaxSize"`
	QuoteMaxSize   string `json:"quoteMaxSize"`
	BaseIncrement  string `json:"baseIncrement"`
	QuoteIncrement string `json:"quoteIncrement"`
	PriceIncrement string `json:"priceIncrement"`
	EnableTrading  bool   `json:"enableTrading"`
}

type OrderRequest struct {
	ClientOid   string          `json:"clientOid"`
	Side        string          `json:"side"`
	Symbol      string          `json:"symbol"`
	Type        string          `json:"type,omitempty"`
	Size        decimal.Decimal `json:"size,omitempty"`
	Price       decimal.Decimal `json:"price,omitempty"`
	Funds       decimal.Decimal `json:"funds,omitempty"`
	TimeInForce string          `json:"timeInForce,omitempty"`
}

type OrderResponse struct {
	OrderId string `json:"orderId"`
}
