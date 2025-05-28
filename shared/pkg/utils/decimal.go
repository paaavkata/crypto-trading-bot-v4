package utils

import (
	"github.com/shopspring/decimal"
)

// Safe float64 to decimal conversion
func FloatToDecimal(val float64) decimal.Decimal {
	return decimal.NewFromFloat(val)
}

// Safe decimal to float64 conversion (may lose precision!)
func DecimalToFloat(val decimal.Decimal) float64 {
	f, _ := val.Float64()
	return f
}

// Parse string, fallback to zero on error
func ParseDecimalSafe(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// Format decimal for exchange API (with 8 decimals)
func DecimalToString(val decimal.Decimal) string {
	return val.StringFixed(8)
}
