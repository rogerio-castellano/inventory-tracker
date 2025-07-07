package repo

import "sort"

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

	// Get average price, total stock value, and total quantity
	totalPrice := 0.0
	totalQuantity := 0
	totalUnitPrice := 0.0
	for _, product := range products {
		totalUnitPrice += product.Price
		totalPrice += product.Price * float64(product.Quantity)
		totalQuantity += product.Quantity
	}
	m.TotalQuantity = totalQuantity
	m.AveragePrice = totalUnitPrice / float64(len(products))
	m.TotalStockValue = totalPrice

	m.Top5Movers = make([]TopMover, 0, 5)

	for _, product := range products {
		_, count, err := i.movementRepo.GetByProductID(product.ID, nil, nil, nil, nil)
		if err != nil {
			return m, err
		}

		// Insert the new mover
		m.Top5Movers = append(m.Top5Movers, TopMover{Name: product.Name, Count: count})

		// Sort and keep only top 5
		sort.Slice(m.Top5Movers, func(i, j int) bool {
			return m.Top5Movers[i].Count > m.Top5Movers[j].Count
		})

		if len(m.Top5Movers) > 5 {
			m.Top5Movers = m.Top5Movers[:5]
		}
	}

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
