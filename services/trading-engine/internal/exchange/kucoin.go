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
