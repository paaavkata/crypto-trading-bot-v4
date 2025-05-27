package main

import (
	"sync"
	"time"
)

const (
	gridLevels     = 10
	gridSpread     = 0.02 // 2% price interval
	gridAllocation = 0.1  // 10% per grid level
)

type Grid struct {
	Levels         []float64
	ActiveBuys     map[float64]bool
	ActiveSells    map[float64]bool
	BasePrice      float64
	CurrentBalance float64
}

type TrendIndicators struct {
	FastEMA *techan.EMAIndicator
	SlowEMA *techan.EMAIndicator
	MACD    *techan.MACDIndicator
	ATR     *techan.ATRIndicator
	BBands  *techan.BollingerBandsIndicator
}

func calculateBollingerBands(series *techan.TimeSeries) *techan.BollingerBandsIndicator {
	closePrices := techan.NewClosePriceIndicator(series)
	sma := techan.NewSMAIndicator(closePrices, 20)
	return techan.NewBollingerBandsIndicator(closePrices, 20, 2.0)
}

func isLowVolatility(bbands *techan.BollingerBandsIndicator) bool {
	upper := bbands.UpperBand().Calculate(0)
	lower := bbands.LowerBand().Calculate(0)
	return (upper-lower)/lower < 0.05 // 5% bandwidth threshold
}

func initializeMACD(series *techan.TimeSeries) *techan.MACDIndicator {
	closePrices := techan.NewClosePriceIndicator(series)
	return techan.NewMACDIndicator(closePrices, 12, 26)
}

func isBullishMACD(macd *techan.MACDIndicator) bool {
	if macd.Histogram().Calculate(1) < 0 &&
		macd.Histogram().Calculate(0) > 0 {
		return true
	}
	return false
}

func calculatePositionSize(atr *techan.ATRIndicator) float64 {
	currentATR := atr.Calculate(0)
	riskPerTrade := 0.02 // 2% of capital
	return (accountBalance * riskPerTrade) / currentATR
}

type TrailingStop struct {
	ActivationPrice float64
	CurrentStop     float64
	ATRMultiplier   float64
}

func updateTrailingStop(ts *TrailingStop, currentPrice, atr float64) {
	if currentPrice > ts.ActivationPrice {
		newStop := currentPrice - (ts.ATRMultiplier * atr)
		if newStop > ts.CurrentStop {
			ts.CurrentStop = newStop
		}
	}
}

func analyzeMarketState(pair TradingPair) MarketState {
	// Technical convergence check
	macdBullish := isBullishMACD(pair.Indicators.MACD)
	emaCross := pair.Indicators.FastEMA.Calculate(0) > pair.Indicators.SlowEMA.Calculate(0)

	// Volatility assessment
	lowVol := isLowVolatility(pair.Indicators.BBands)

	// Volume analysis
	volumeSpike := detectVolumeSpike(pair.VolumeHistory)

	if lowVol && !volumeSpike {
		return RANGING
	}
	if macdBullish && emaCross {
		return TRENDING_UP
	}
	return TRENDING_DOWN
}

func handleGridExecution(pair TradingPair, price float64) bool {
	grid := pair.GridStrategy
	currentLevel := findNearestGridLevel(price, grid.Levels)

	if price > grid.BasePrice {
		if grid.ActiveSells[currentLevel] {
			executeOrder(pair.Symbol, SELL, gridAllocation)
			grid.ActiveSells[currentLevel] = false
			placeLowerBuyOrder(currentLevel)
			return true
		}
	} else {
		if grid.ActiveBuys[currentLevel] {
			executeOrder(pair.Symbol, BUY, gridAllocation)
			grid.ActiveBuys[currentLevel] = false
			placeHigherSellOrder(currentLevel)
			return true
		}
	}
	return false
}

var (
	orderMutex    sync.RWMutex
	pendingOrders map[string]bool
)

func executeOrderSafe(symbol string, side OrderSide, amount float64) {
	orderMutex.Lock()
	defer orderMutex.Unlock()

	if !pendingOrders[symbol] {
		go func() {
			pendingOrders[symbol] = true
			executeOrder(symbol, side, amount)
			pendingOrders[symbol] = false
		}()
	}
}

type RateLimiter struct {
	requests chan time.Time
}

func NewRateLimiter(interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(chan time.Time, burst),
	}
	go func() {
		for t := range time.Tick(interval) {
			select {
			case rl.requests <- t:
			default:
			}
		}
	}()
	return rl
}

func (rl *RateLimiter) Wait() {
	<-rl.requests
}
