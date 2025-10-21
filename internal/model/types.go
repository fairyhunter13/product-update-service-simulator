// Package model defines domain types used by the service.
package model

// Event represents an incoming product update event.
type Event struct {
	ProductID string   `json:"product_id"`
	Price     *float64 `json:"price,omitempty"`
	Stock     *int64   `json:"stock,omitempty"`
	Sequence  uint64   `json:"-"`
}

// Product represents the current state of a product.
type Product struct {
	ProductID string  `json:"product_id"`
	Price     float64 `json:"price"`
	Stock     int64   `json:"stock"`
}
