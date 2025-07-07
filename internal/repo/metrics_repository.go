package repo

type MostMovedProduct struct {
	Name          string `json:"name"`
	MovementCount int    `json:"movement_count"`
}

type TopMover struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Metrics struct {
	TotalProducts    int              `json:"total_products"`
	TotalMovements   int              `json:"total_movements"`
	LowStockCount    int              `json:"low_stock_count"`
	MostMovedProduct MostMovedProduct `json:"most_moved_product"`
	AveragePrice     float64          `json:"average_price"`
	TotalStockValue  float64          `json:"total_stock_value"`
	TotalQuantity    int              `json:"total_quantity"`
	Top5Movers       []TopMover       `json:"top_5_movers"`
}

type MetricsRepository interface {
	GetDashboardMetrics() (Metrics, error)
}
