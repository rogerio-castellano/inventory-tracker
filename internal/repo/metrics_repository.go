package repo

type MostMovedProduct struct {
	Name          string `json:"name"`
	MovementCount int    `json:"movement_count"`
}

type Metrics struct {
	TotalProducts    int              `json:"total_products"`
	TotalMovements   int              `json:"total_movements"`
	LowStockCount    int              `json:"low_stock_count"`
	MostMovedProduct MostMovedProduct `json:"most_moved_product"`
}

type MetricsRepository interface {
	GetDashboardMetrics() (Metrics, error)
}
