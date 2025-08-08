package handlers

import "time"

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

type LoginResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RegisterResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type ImportProductsResult struct {
	ImportedProductsCount int                      `json:"imported"`
	Errors                []ProductValidationError `json:"errors"`
}

type CredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterAsAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"` // e.g., "user" or "admin"
}

type MeResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type RefreshRequest struct {
	Username     string `json:"username"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenInfo contains metadata for auditing refresh tokens
type RefreshTokenInfo struct {
	Username  string    `json:"username"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
}
