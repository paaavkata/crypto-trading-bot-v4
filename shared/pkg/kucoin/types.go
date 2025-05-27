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
