package kucoin

import (
	"time"
)

type APIResponse struct {
	Code string      `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type AllTickersResponse struct {
	Time   int64    `json:"time"`
	Ticker []Ticker `json:"ticker"`
}

type Ticker struct {
	Symbol      string `json:"symbol"`
	SymbolName  string `json:"symbolName"`
	Buy         string `json:"buy"`
	Sell        string `json:"sell"`
	ChangeRate  string `json:"changeRate"`
	ChangePrice string `json:"changePrice"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Vol         string `json:"vol"`
	VolValue    string `json:"volValue"`
	Last        string `json:"last"`
	AveragePrice string `json:"averagePrice"`
	TakerFeeRate string `json:"takerFeeRate"`
	MakerFeeRate string `json:"makerFeeRate"`
	TakerCoefficient string `json:"takerCoefficient"`
	MakerCoefficient string `json:"makerCoefficient"`
}

type Symbol struct {
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	BaseCurrency    string `json:"baseCurrency"`
	QuoteCurrency   string `json:"quoteCurrency"`
	FeeCurrency     string `json:"feeCurrency"`
	Market          string `json:"market"`
	BaseMinSize     string `json:"baseMinSize"`
	QuoteMinSize    string `json:"quoteMinSize"`
	BaseMaxSize     string `json:"baseMaxSize"`
	QuoteMaxSize    string `json:"quoteMaxSize"`
	BaseIncrement   string `json:"baseIncrement"`
	QuoteIncrement  string `json:"quoteIncrement"`
	PriceIncrement  string `json:"priceIncrement"`
	PriceLimitRate  string `json:"priceLimitRate"`
	MinFunds        string `json:"minFunds"`
	IsMarginEnabled bool   `json:"isMarginEnabled"`
	EnableTrading   bool   `json:"enableTrading"`
}

type OrderRequest struct {
	ClientOid string `json:"clientOid"`
	Side      string `json:"side"`
	Symbol    string `json:"symbol"`
	Type      string `json:"type,omitempty"`
	Remark    string `json:"remark,omitempty"`
	Stop      string `json:"stop,omitempty"`
	StopPrice string `json:"stopPrice,omitempty"`
	Stp       string `json:"stp,omitempty"`
	TradeType string `json:"tradeType,omitempty"`
	Price     string `json:"price,omitempty"`
	Size      string `json:"size,omitempty"`
	Funds     string `json:"funds,omitempty"`
	TimeInForce string `json:"timeInForce,omitempty"`
	CancelAfter int64  `json:"cancelAfter,omitempty"`
	PostOnly    bool   `json:"postOnly,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	Iceberg     bool   `json:"iceberg,omitempty"`
	VisibleSize string `json:"visibleSize,omitempty"`
}

type OrderResponse struct {
	OrderId string `json:"orderId"`
}

type KlineData struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	Close     float64 `json:"close"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Volume    float64 `json:"volume"`
}

type AccountInfo struct {
	Accounts []Account `json:"accounts"`
}

type Account struct {
	Id        string `json:"id"`
	Currency  string `json:"currency"`
	Type      string `json:"type"`
	Balance   string `json:"balance"`
	Available string `json:"available"`
	Holds     string `json:"holds"`
}

type OrderStatus struct {
	Id            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	OpType        string    `json:"opType"`
	Type          string    `json:"type"`
	Side          string    `json:"side"`
	Amount        string    `json:"amount"`
	Funds         string    `json:"funds"`
	DealFunds     string    `json:"dealFunds"`
	DealSize      string    `json:"dealSize"`
	Fee           string    `json:"fee"`
	FeeCurrency   string    `json:"feeCurrency"`
	Stp           string    `json:"stp"`
	Stop          string    `json:"stop"`
	StopTriggered bool      `json:"stopTriggered"`
	StopPrice     string    `json:"stopPrice"`
	TimeInForce   string    `json:"timeInForce"`
	PostOnly      bool      `json:"postOnly"`
	Hidden        bool      `json:"hidden"`
	Iceberg       bool      `json:"iceberg"`
	VisibleSize   string    `json:"visibleSize"`
	CancelAfter   int64     `json:"cancelAfter"`
	Channel       string    `json:"channel"`
	ClientOid     string    `json:"clientOid"`
	Remark        string    `json:"remark"`
	Tags          string    `json:"tags"`
	IsActive      bool      `json:"isActive"`
	CancelExist   bool      `json:"cancelExist"`
	CreatedAt     int64     `json:"createdAt"`
	TradeType     string    `json:"tradeType"`
}