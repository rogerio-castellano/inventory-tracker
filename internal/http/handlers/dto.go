package handlers

type ProductRequest struct {
	Id        int     `json:"id,omitempty"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Threshold int     `json:"threshold"`
}

type ProductResponse struct {
	Id        int     `json:"id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Threshold int     `json:"threshold"`
	LowStock  bool    `json:"low_stock,omitempty"`
}

type Meta struct {
	TotalCount int `json:"total_count"`
}

type ProductsSearchResult struct {
	Data []ProductResponse `json:"data"`
	Meta Meta              `json:"meta,omitempty"`
}

type QuantityAdjustmentRequest struct {
	Delta int `json:"delta"` // can be positive or negative
}

type MovementResponse struct {
	ID        int    `json:"id"`
	ProductID int    `json:"product_id"`
	Delta     int    `json:"delta"`
	CreatedAt string `json:"created_at"`
}

type MovementsSearchResult struct {
	Data []MovementResponse `json:"data"`
	Meta Meta               `json:"meta,omitempty"`
}

type UserLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Token string `json:"token"`
}

type RegisterResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type ImportProductsResult struct {
	ImportedProductsCount int                      `json:"imported"`
	Errors                []ProductValidationError `json:"errors"`
}
