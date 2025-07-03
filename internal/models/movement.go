package models

type Movement struct {
	ID        int    `json:"id"`
	ProductID int    `json:"product_id"`
	Delta     int    `json:"delta"`
	Timestamp string `json:"timestamp"`
}
