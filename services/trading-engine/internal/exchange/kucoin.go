package exchange

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
	"github.com/sirupsen/logrus"
)

type KuCoinExchange struct {
	client *kucoin.Client
	logger *logrus.Logger
}

func NewKuCoinExchange(client *kucoin.Client, logger *logrus.Logger) *KuCoinExchange {
	return &KuCoinExchange{
		client: client,
		logger: logger,
	}
}

func (k *KuCoinExchange) PlaceBuyOrder(symbol string, quantity, price float64) (*kucoin.OrderResponse, error) {
	clientOid := uuid.New().String()

	order := kucoin.OrderRequest{
		ClientOid:   clientOid,
		Side:        "buy",
		Symbol:      symbol,
		Type:        "limit",
		Size:        strconv.FormatFloat(quantity, 'f', 8, 64),
		Price:       strconv.FormatFloat(price, 'f', 8, 64),
		TimeInForce: "GTC",
	}

	k.logger.WithFields(logrus.Fields{
		"symbol":     symbol,
		"side":       "buy",
		"quantity":   quantity,
		"price":      price,
		"client_oid": clientOid,
	}).Info("Placing buy order")

	return k.client.PlaceOrder(order)
}

func (k *KuCoinExchange) GetOrderStatus(ctx context.Context, orderID string) (*models.OrderDetail, error) {
	kucoinOrder, err := k.client.GetOrder(ctx, orderID)
	if err != nil {
		// Specific error check for "order not found" might be useful here if the shared client wraps it.
		// For now, we log and return.
		k.logger.WithError(err).WithField("kucoin_order_id", orderID).Error("Failed to get order status from KuCoin shared client")
		return nil, fmt.Errorf("failed to get order status for %s: %w", orderID, err)
	}

	// Helper function to parse string to float64, returns 0 on error for simplicity here
	// In a real scenario, handle parsing errors more robustly.
	parseFloat := func(s string) float64 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	// Map shared.KucoinOrderDetail to models.OrderDetail
	orderDetail := &models.OrderDetail{
		ID:          kucoinOrder.ID,
		ClientOid:   kucoinOrder.ClientOid,
		Symbol:      kucoinOrder.Symbol,
		Side:        kucoinOrder.Side,
		Type:        kucoinOrder.Type,
		Price:       parseFloat(kucoinOrder.Price),
		Size:        parseFloat(kucoinOrder.Size),
		DealFunds:   parseFloat(kucoinOrder.DealFunds),
		DealSize:    parseFloat(kucoinOrder.DealSize),
		Fee:         parseFloat(kucoinOrder.Fee),
		FeeCurrency: kucoinOrder.FeeCurrency,
		IsActive:    kucoinOrder.IsActive,
		Status:      mapKucoinStatusToInternal(kucoinOrder), // Standardize status
		CreatedAt:   time.UnixMilli(kucoinOrder.CreatedAt),
		UpdatedAt:   time.Now(), // Set UpdatedAt to now, as we just fetched it.
	}

	k.logger.WithFields(logrus.Fields{
		"kucoin_order_id": orderID,
		"internal_status": orderDetail.Status,
		"deal_size":       orderDetail.DealSize,
		"is_active":       orderDetail.IsActive,
	}).Debug("Successfully mapped Kucoin order detail to internal model")

	return orderDetail, nil
}

// mapKucoinStatusToInternal standardizes Kucoin order status.
// Kucoin's GetOrder endpoint primarily uses `isActive`. Other endpoints might provide more granular statuses.
func mapKucoinStatusToInternal(detail *kucoin.KucoinOrderDetail) string {
	// If the shared client already inferred a status, use it.
	if detail.Status != "" && detail.Status != "unknown" { // "unknown" was a default in shared client if logic failed
		// Potentially further map specific Kucoin statuses if shared client's inference is too basic
		switch detail.Status {
		case "active": // From shared client's inference
			if detail.DealSize > 0 && detail.DealSize < parseFloat(detail.Size) {
				return "partially_filled"
			}
			return "open" // or "pending" if it means not yet on order book. Kucoin's 'active' means on order book.
		case "filled": // From shared client's inference
			return "filled"
		case "canceled": // From shared client's inference
			return "canceled"
		}
	}

	// Fallback logic if shared client's status is not definitive enough or absent
	if detail.IsActive {
		if detail.DealSize > 0 && detail.DealSize < parseFloat(detail.Size) {
			return "partially_filled"
		}
		// If it's active and no deal size, it's open/pending.
		// If it's active and some deal size, it's partially filled.
		return "open" // Or "pending" depending on precise definition desired
	}

	// Not active: could be filled, canceled, or rarely, rejected (though rejection often happens on placement)
	if detail.DealSize > 0 && detail.DealSize == parseFloat(detail.Size) {
		return "filled"
	}
	if detail.CancelExist == false && detail.DealSize < parseFloat(detail.Size) {
		// This combination often means it was canceled by exchange or user.
		// If DealSize > 0, it was partially filled then canceled.
		if detail.DealSize > 0 {
			return "canceled_partially_filled" // Custom status for clarity
		}
		return "canceled"
	}

	// If no specific status can be determined, treat as error or unknown for re-evaluation
	return "unknown"
}

// Helper for mapKucoinStatusToInternal, as it's used there.
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (k *KuCoinExchange) PlaceSellOrder(symbol string, quantity, price float64) (*kucoin.OrderResponse, error) {
	clientOid := uuid.New().String()

	order := kucoin.OrderRequest{
		ClientOid:   clientOid,
		Side:        "sell",
		Symbol:      symbol,
		Type:        "limit",
		Size:        strconv.FormatFloat(quantity, 'f', 8, 64),
		Price:       strconv.FormatFloat(price, 'f', 8, 64),
		TimeInForce: "GTC",
	}

	k.logger.WithFields(logrus.Fields{
		"symbol":     symbol,
		"side":       "sell",
		"quantity":   quantity,
		"price":      price,
		"client_oid": clientOid,
	}).Info("Placing sell order")

	return k.client.PlaceOrder(order)
}

func (k *KuCoinExchange) PlaceMarketOrder(symbol, side string, quantity float64) (*kucoin.OrderResponse, error) {
	clientOid := uuid.New().String()

	order := kucoin.OrderRequest{
		ClientOid: clientOid,
		Side:      side,
		Symbol:    symbol,
		Type:      "market",
		Size:      strconv.FormatFloat(quantity, 'f', 8, 64),
	}

	k.logger.WithFields(logrus.Fields{
		"symbol":     symbol,
		"side":       side,
		"quantity":   quantity,
		"type":       "market",
		"client_oid": clientOid,
	}).Info("Placing market order")

	return k.client.PlaceOrder(order)
}
