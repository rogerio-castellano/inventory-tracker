package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
)

// ImportProductsHandler godoc
// @Summary Import products via CSV
// @Tags import
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV file"
// @Param mode query string false "Import mode (skip|update)"
// @Success 200 {object} map[string]any
// @Failure 400 {string} string "Invalid file"
// @Failure 500 {string} string "Internal error"
// @Router /products/import [post]
// @Security BearerAuth
func ImportProductsHandler(w http.ResponseWriter, r *http.Request) {
	mode := strings.ToLower(r.URL.Query().Get("mode"))
	if mode != "update" {
		mode = "skip" // default
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		http.Error(w, "invalid CSV header", http.StatusBadRequest)
		return
	}

	headerIndex := map[string]int{}
	for i, h := range headers {
		headerIndex[strings.ToLower(h)] = i
	}

	var imported int
	var errorsList []ProductValidationError

	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: %v", row, err)})
			continue
		}

		name := record[headerIndex["name"]]
		price, _ := strconv.ParseFloat(record[headerIndex["price"]], 64)
		quantity, _ := strconv.Atoi(record[headerIndex["quantity"]])
		threshold, _ := strconv.Atoi(record[headerIndex["threshold"]])

		if strings.TrimSpace(name) == "" || price <= 0 || quantity < 0 || threshold < 0 {
			errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: invalid values", row)})
			continue
		}

		// Check if product with same name exists
		existing, err := productRepo.GetByName(name)
		if err == nil && existing.ID != 0 {
			if mode == "skip" {
				errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: product '%s' already exists", row, name)})
				continue
			}
		}

		if mode == "update" {
			existing.Price = price
			existing.Quantity = quantity
			existing.Threshold = threshold
			existing.UpdatedAt = time.Now().Format(time.RFC3339)

			_, err = productRepo.Update(existing)
			if err != nil {
				errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: failed to update '%s'", row, name)})
				continue
			}
			imported++
			continue
		}

		product := models.Product{
			Name:      name,
			Price:     price,
			Quantity:  quantity,
			Threshold: threshold,
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
		}

		_, err = productRepo.Create(product)
		if err != nil {
			errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: %v", row, err)})
			continue
		}
		imported++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ImportProductsResult{
		ImportedProductsCount: imported,
		Errors:                errorsList,
	})
}
