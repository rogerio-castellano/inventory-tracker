package repo

type InMemoryMetricsRepository struct {
	productRepo  ProductRepository
	movementRepo MovementRepository
}

// GetDashboardMetrics implements MetricsRepository.
func (i *InMemoryMetricsRepository) GetDashboardMetrics() (Metrics, error) {
	m := Metrics{}

	// Get total products
	products, err := i.productRepo.GetAll()
	if err != nil {
		return m, err
	}
	m.TotalProducts = len(products)

	// Get total movements
	for _, product := range products {
		_, count, err := i.movementRepo.GetByProductID(product.ID, nil, nil, nil, nil)
		if err != nil {
			return m, err
		}
		m.TotalMovements += count
		// Get most moved product
		if count > m.MostMovedProduct.MovementCount {
			m.MostMovedProduct.Name = product.Name
			m.MostMovedProduct.MovementCount = count
		}
	}

	// Get low stock count
	for _, product := range products {
		if product.Quantity < product.Threshold {
			m.LowStockCount++
		}
	}

	// if len(movements) > 0 {
	// 	productMovementCount := make(map[string]int)
	// 	for _, movement := range movements {
	// 		productMovementCount[movement.ProductName]++
	// 	}

	// 	for name, count := range productMovementCount {
	// 		if count > m.MostMovedProduct.MovementCount {
	// 			m.MostMovedProduct.Name = name
	// 			m.MostMovedProduct.MovementCount = count
	// 		}
	// 	}
	// }

	return m, nil
}

func NewInMemoryMetricsRepository() *InMemoryMetricsRepository {
	return &InMemoryMetricsRepository{}
}

func (i *InMemoryMetricsRepository) SetRepositories(
	productRepo ProductRepository,
	movementRepo MovementRepository,
) {
	i.productRepo = productRepo
	i.movementRepo = movementRepo
}
