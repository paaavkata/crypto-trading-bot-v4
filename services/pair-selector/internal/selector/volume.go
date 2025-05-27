package selector

import (
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/sirupsen/logrus"
)

type VolumeAnalyzer struct {
	logger *logrus.Logger
}

type VolumeMetrics struct {
	Volume24hUSDT     float64
	AverageVolume     float64
	VolumeConsistency float64
}

func NewVolumeAnalyzer(logger *logrus.Logger) *VolumeAnalyzer {
	return &VolumeAnalyzer{logger: logger}
}

func (v *VolumeAnalyzer) AnalyzeVolume(priceData []models.PricePoint) VolumeMetrics {
	if len(priceData) == 0 {
		return VolumeMetrics{}
	}

	totalVolume := 0.0
	volumes := make([]float64, len(priceData))

	for i, point := range priceData {
		totalVolume += point.Volume * point.Close // Convert to USDT value
		volumes[i] = point.Volume * point.Close
	}

	averageVolume := totalVolume / float64(len(priceData))

	// Calculate volume consistency (inverse of coefficient of variation)
	consistency := v.calculateVolumeConsistency(volumes, averageVolume)

	return VolumeMetrics{
		Volume24hUSDT:     totalVolume,
		AverageVolume:     averageVolume,
		VolumeConsistency: consistency,
	}
}

func (v *VolumeAnalyzer) calculateVolumeConsistency(volumes []float64, average float64) float64 {
	if len(volumes) == 0 || average == 0 {
		return 0
	}

	variance := 0.0
	for _, vol := range volumes {
		variance += (vol - average) * (vol - average)
	}
	variance /= float64(len(volumes))

	stdDev := variance
	if variance > 0 {
		stdDev = variance * variance // Simplified square root approximation
	}

	coefficientOfVariation := stdDev / average

	// Return inverse of coefficient of variation (higher is more consistent)
	if coefficientOfVariation > 0 {
		return 1.0 / coefficientOfVariation
	}
	return 1.0
}
