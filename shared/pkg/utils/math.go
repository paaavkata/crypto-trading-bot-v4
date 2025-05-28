package utils

import (
	"math"
	"strconv"
)

func ParseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

func CalculateATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period || len(lows) < period || len(closes) < period {
		return 0
	}

	var trueRanges []float64

	for i := 1; i < len(highs); i++ {
		tr1 := highs[i] - lows[i]
		tr2 := math.Abs(highs[i] - closes[i-1])
		tr3 := math.Abs(lows[i] - closes[i-1])

		trueRange := math.Max(tr1, math.Max(tr2, tr3))
		trueRanges = append(trueRanges, trueRange)
	}

	if len(trueRanges) < period {
		return 0
	}

	// Calculate simple moving average of true ranges
	sum := 0.0
	for i := len(trueRanges) - period; i < len(trueRanges); i++ {
		sum += trueRanges[i]
	}

	return sum / float64(period)
}

func CalculateVolatility(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}

	var returns []float64
	for i := 1; i < len(prices); i++ {
		if prices[i-1] != 0 {
			ret := (prices[i] - prices[i-1]) / prices[i-1]
			returns = append(returns, ret)
		}
	}

	if len(returns) == 0 {
		return 0
	}

	// Calculate mean
	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	// Calculate variance
	variance := 0.0
	for _, ret := range returns {
		variance += math.Pow(ret-mean, 2)
	}
	variance /= float64(len(returns))

	// Return standard deviation (volatility)
	return math.Sqrt(variance)
}

func CalculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	var sumX, sumY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation coefficient
	var numerator, denomX, denomY float64
	for i := 0; i < len(x); i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		denomX += diffX * diffX
		denomY += diffY * diffY
	}

	if denomX == 0 || denomY == 0 {
		return 0
	}

	return numerator / math.Sqrt(denomX*denomY)
}

func NormalizeTo(value float64, decimalPlaces int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0.0
	}

	multiplier := math.Pow(10, float64(decimalPlaces))
	return math.Round(value*multiplier) / multiplier
}

// CapValue caps a value to a maximum while preserving sign
func CapValue(value, maxValue float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0.0
	}

	if value > maxValue {
		return maxValue
	}
	if value < -maxValue {
		return -maxValue
	}
	return value
}

// NormalizeDecimal normalizes a value to fit within PostgreSQL DECIMAL constraints
func NormalizeDecimal(value float64, totalDigits, decimalPlaces int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0.0
	}

	// Calculate maximum value based on total digits and decimal places
	integerDigits := totalDigits - decimalPlaces
	maxValue := math.Pow(10, float64(integerDigits)) - math.Pow(10, -float64(decimalPlaces))

	// Cap the value
	cappedValue := CapValue(value, maxValue)

	// Round to specified decimal places
	return NormalizeTo(cappedValue, decimalPlaces)
}
