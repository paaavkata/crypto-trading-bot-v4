package models

import (
	"time"
)

type PriceData struct {
	ID          int64     `db:"id"`
	Symbol      string    `db:"symbol"`
	Timestamp   time.Time `db:"timestamp"`
	Open        float64   `db:"open"`
	High        float64   `db:"high"`
	Low         float64   `db:"low"`
	Close       float64   `db:"close"`
	Volume      float64   `db:"volume"`
	QuoteVolume float64   `db:"quote_volume"`
	ChangeRate  float64   `db:"change_rate"`
	ChangePrice float64   `db:"change_price"`
	CreatedAt   time.Time `db:"created_at"`
}

type TickerData struct {
	Symbol      string    `json:"symbol"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	QuoteVolume float64   `json:"quote_volume"`
	ChangeRate  float64   `json:"change_rate"`
	ChangePrice float64   `json:"change_price"`
	Timestamp   time.Time `json:"timestamp"`
}
