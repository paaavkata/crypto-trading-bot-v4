package kucoin

type APIResponse struct {
	Code string      `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type Ticker struct {
	Symbol       string `json:"symbol"`
	SymbolName   string `json:"symbolName"`
	Buy          string `json:"buy"`
	Sell         string `json:"sell"`
	ChangeRate   string `json:"changeRate"`
	ChangePrice  string `json:"changePrice"`
	High         string `json:"high"`
	Low          string `json:"low"`
	Vol          string `json:"vol"`
	VolValue     string `json:"volValue"`
	Last         string `json:"last"`
	AveragePrice string `json:"averagePrice"`
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
	ClientOid   string `json:"clientOid"`
	Side        string `json:"side"`
	Symbol      string `json:"symbol"`
	Type        string `json:"type,omitempty"`
	Size        string `json:"size,omitempty"`
	Price       string `json:"price,omitempty"`
	Funds       string `json:"funds,omitempty"`
	TimeInForce string `json:"timeInForce,omitempty"`
}

type OrderResponse struct {
	OrderId string `json:"orderId"`
}

// KucoinOrderDetail represents the detailed information of an order from Kucoin.
// Fields are based on common structures; refer to actual Kucoin API docs for exact fields.
type KucoinOrderDetail struct {
	ID            string `json:"id"` // Order ID
	ClientOid     string `json:"clientOid"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`  // "buy" or "sell"
	Type          string `json:"type"`  // "limit", "market", "stop_limit", etc.
	Price         string `json:"price"` // Original order price
	Size          string `json:"size"`  // Original order size (quantity)
	DealFunds     string `json:"dealFunds"` // Amount executed in quote currency
	DealSize      string `json:"dealSize"`  // Amount executed in base currency (filled quantity)
	Fee           string `json:"fee"`
	FeeCurrency   string `json:"feeCurrency"`
	Stp           string `json:"stp"`
	Stop          string `json:"stop"`
	StopTriggered bool   `json:"stopTriggered"`
	StopPrice     string `json:"stopPrice"`
	TimeInForce   string `json:"timeInForce"`
	PostOnly      bool   `json:"postOnly"`
	Hidden        bool   `json:"hidden"`
	Iceberg       bool   `json:"iceberg"`
	VisibleSize   string `json:"visibleSize"`
	CancelAfter   int64  `json:"cancelAfter"`
	Channel       string `json:"channel"`
	Tags          string `json:"tags"`
	IsActive      bool   `json:"isActive"`     // true if the order is active, false if filled or cancelled
	CancelExist   bool   `json:"cancelExist"`  // true if the order can be cancelled
	CreatedAt     int64  `json:"createdAt"`    // Order creation time (milliseconds)
	TradeType     string `json:"tradeType"`    // e.g., TRADE, MARGIN_TRADE
	Status        string `json:"status"`       // This field is not directly in Kucoin's /api/v1/orders/{orderId} but isActive is. We might infer status.
											  // Common statuses from other endpoints or general knowledge: "active", "done", "pending"
											  // Kucoin's "done" status is typically inferred from isActive=false and no cancelExist.
}
