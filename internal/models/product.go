package models

// Product represents a product entity in the inventory system.
type Product struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at,omitempty"`
	UpdatedAt string  `json:"updated_at,omitempty"`
}
