package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/api"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/common/logger"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/spot/market"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/types"
	"github.com/gorilla/websocket"
)

const (
	pairRefreshInterval = 5 * time.Minute
	tradingPairLimit    = 8
	volatilityThreshold = 0.05    // 5% daily volatility
	minDailyVolume      = 1000000 // $1M USDT
)

type TradingPair struct {
	Symbol      string
	LastPrice   float64
	Volatility  float64
	DailyVolume float64
}

var (
	activePairs  = make(map[string]TradingPair)
	pairMux      sync.RWMutex
	shutdownChan = make(chan struct{})
)

func main() {
	// Initialize logging
	logger.SetLogger(logger.NewDefaultLogger())

	// Initialize KuCoin client
	client := initKucoinClient()

	// Start pair selection component
	go runPairSelection(client)

	// Start trading component
	go runTradingEngine(client)

	// Block until shutdown signal
	<-shutdownChan
}

func initKucoinClient() *api.DefaultClient {
	key := os.Getenv("KUCOIN_API_KEY")
	secret := os.Getenv("KUCOIN_API_SECRET")
	passphrase := os.Getenv("KUCOIN_API_PASSPHRASE")

	option := types.NewClientOptionBuilder().
		WithKey(key).
		WithSecret(secret).
		WithPassphrase(passphrase).
		WithSpotEndpoint(types.GlobalApiEndpoint).
		Build()

	return api.NewClient(option)
}

// Pair Selection Component
func runPairSelection(client *api.DefaultClient) {
	ticker := time.NewTicker(pairRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updateTradingPairs(client)
		case <-shutdownChan:
			return
		}
	}
}

func updateTradingPairs(client *api.DefaultClient) {
	// Get all trading pairs
	marketAPI := client.RestService().GetSpotService().GetMarketAPI()
	req := market.NewGetAllTickersReqBuilder().Build()
	resp, err := marketAPI.GetAllTickers(req, context.Background())
	if err != nil {
		log.Printf("Error fetching tickers: %v", err)
		return
	}

	pairMux.Lock()
	defer pairMux.Unlock()

	for _, ticker := range resp.Tickers {
		symbol := ticker.Symbol
		if !isUsdtPair(symbol) {
			continue
		}

		volatility := calculateVolatility(ticker)
		dailyVolume := parseFloat(ticker.VolValue)

		if meetsCriteria(volatility, dailyVolume) {
			activePairs[symbol] = TradingPair{
				Symbol:      symbol,
				LastPrice:   parseFloat(ticker.LastPrice),
				Volatility:  volatility,
				DailyVolume: dailyVolume,
			}
		}
	}

	// Enforce pair limit
	if len(activePairs) > tradingPairLimit {
		trimPairs()
	}
}

// Trading Component
func runTradingEngine(client *api.DefaultClient) {
	wsClient := initWebsocketClient(client)
	defer wsClient.Close()

	priceChan := make(chan PriceUpdate)
	go processPriceUpdates(priceChan)

	for {
		select {
		case <-shutdownChan:
			return
		default:
			handleWebsocketMessages(wsClient, priceChan)
		}
	}
}

func initWebsocketClient(client *api.DefaultClient) *websocket.Conn {
	// Get WS token
	tokenResp, err := client.RestService().GetWebsocketToken(context.Background())
	if err != nil {
		log.Fatalf("Error getting WS token: %v", err)
	}

	// Connect to WS endpoint
	wsURL := tokenResp.InstanceServers[0].Endpoint + "?token=" + tokenResp.Token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("WS connection error: %v", err)
	}

	// Subscribe to active pairs
	pairMux.RLock()
	symbols := make([]string, 0, len(activePairs))
	for symbol := range activePairs {
		symbols = append(symbols, symbol)
	}
	pairMux.RUnlock()

	subMsg := map[string]interface{}{
		"id":             time.Now().UnixMilli(),
		"type":           "subscribe",
		"topic":          "/market/ticker:" + joinSymbols(symbols),
		"response":       true,
		"privateChannel": false,
	}

	if err := conn.WriteJSON(subMsg); err != nil {
		log.Fatalf("Subscription error: %v", err)
	}

	return conn
}

func handleWebsocketMessages(conn *websocket.Conn, priceChan chan PriceUpdate) {
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("WS read error: %v", err)
		return
	}

	if msg.Type == "message" && msg.Topic == "/market/ticker" {
		priceChan <- PriceUpdate{
			Symbol:    msg.Data.Symbol,
			Price:     parseFloat(msg.Data.Price),
			Timestamp: msg.Data.Timestamp,
		}
	}
}

func processPriceUpdates(priceChan chan PriceUpdate) {
	for update := range priceChan {
		pairMux.RLock()
		pair, exists := activePairs[update.Symbol]
		pairMux.RUnlock()

		if !exists {
			continue
		}

		// Execute trading strategy
		if shouldExecuteOrder(pair, update.Price) {
			executeOrder(pair.Symbol, update.Price)
		}
	}
}

// Helper functions
func isUsdtPair(symbol string) bool {
	return len(symbol) > 5 && symbol[len(symbol)-5:] == "-USDT"
}

func calculateVolatility(ticker market.Ticker) float64 {
	high := parseFloat(ticker.High)
	low := parseFloat(ticker.Low)
	return (high - low) / low
}

func meetsCriteria(volatility, volume float64) bool {
	return volatility >= volatilityThreshold && volume >= minDailyVolume
}

func executeOrder(symbol string, price float64) {
	// Implement order execution logic using KuCoin API
	// Example: client.RestService().GetSpotService().CreateOrder(...)
}

// Data structures
type PriceUpdate struct {
	Symbol    string
	Price     float64
	Timestamp int64
}

type WSMessage struct {
	Type  string `json:"type"`
	Topic string `json:"topic"`
	Data  struct {
		Symbol    string `json:"symbol"`
		Price     string `json:"price"`
		Timestamp int64  `json:"time"`
	} `json:"data"`
}

func parseFloat(value string) float64 {
	var f float64
	if _, err := fmt.Sscanf(value, "%f", &f); err != nil {
		log.Printf("Error parsing float: %v", err)
		return 0.0
	}
	return f
}

func joinSymbols(symbols []string) string {
	if len(symbols) == 0 {
		return ""
	}
	return fmt.Sprintf("%s", symbols)
}

func trimPairs() {
	pairMux.Lock()
	defer pairMux.Unlock()

	// Sort pairs by daily volume and remove the lowest ones
	var pairs []TradingPair
	for _, pair := range activePairs {
		pairs = append(pairs, pair)
	}

	// Sort by daily volume descending
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].DailyVolume > pairs[j].DailyVolume
	})

	if len(pairs) > tradingPairLimit {
		activePairs = make(map[string]TradingPair)
		for i := 0; i < tradingPairLimit; i++ {
			activePairs[pairs[i].Symbol] = pairs[i]
		}
	}
}

func shouldExecuteOrder(pair TradingPair, price float64) bool {
	// Get current market state
	marketState := analyzeMarketState(pair)

	// Hybrid execution logic
	switch marketState {
	case TRENDING_UP:
		return handleTrendEntry(pair, price, LONG)
	case TRENDING_DOWN:
		return handleTrendEntry(pair, price, SHORT)
	case RANGING:
		return handleGridExecution(pair, price)
	default:
		return false
	}
}
