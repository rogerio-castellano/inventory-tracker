// internal/models/filter.go
package repo

type ProductFilter struct {
	Name     string
	MinPrice *float64
	MaxPrice *float64
	MinQty   *int
	MaxQty   *int
	Offset   *int
	Limit    *int
}
