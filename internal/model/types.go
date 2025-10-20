package model

type Event struct {
	ProductID string   `json:"product_id"`
	Price     *float64 `json:"price,omitempty"`
	Stock     *int64   `json:"stock,omitempty"`
	Sequence  uint64   `json:"-"`
}

type Product struct {
	ProductID string  `json:"product_id"`
	Price     float64 `json:"price"`
	Stock     int64   `json:"stock"`
}
