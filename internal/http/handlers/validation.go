package handlers

import (
	"strings"
)

type ProductValidationError struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

func validateProduct(p ProductRequest) []ProductValidationError {
	errs := []ProductValidationError{}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, ProductValidationError{Field: "Name", Description: "Name is required"})
	}
	if p.Price <= 0 {
		errs = append(errs, ProductValidationError{Field: "Price", Description: "Price must be greater than zero"})
	}
	if p.Quantity < 0 {
		errs = append(errs, ProductValidationError{Field: "Quantity", Description: "Quantity cannot be negative"})
	}
	return errs
}
