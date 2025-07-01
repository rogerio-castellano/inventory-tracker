package repo

// ProductRepository defines the interface for product data operations.
type ProductRepository interface {
	Create(product Product) (Product, error)
	GetAll() ([]Product, error)
	GetByID(id int) (Product, error)
	Update(product Product) (Product, error)
	Delete(id int) error
}
