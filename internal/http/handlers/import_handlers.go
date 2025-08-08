package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type csvRow struct {
	Name      string
	Price     float64
	Quantity  int
	Threshold int
}

func parseCSV(file multipart.File) ([]csvRow, error) {
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV header")
	}

	index := map[string]int{}
	for i, h := range headers {
		index[strings.ToLower(h)] = i
	}

	var rows []csvRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV read error: %v", err)
		}

		row := csvRow{
			Name:      record[index["name"]],
			Price:     parseFloat(record[index["price"]]),
			Quantity:  parseInt(record[index["quantity"]]),
			Threshold: parseInt(record[index["threshold"]]),
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateRow(r csvRow) error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("missing name")
	}
	if r.Price <= 0 {
		return errors.New("invalid price")
	}
	if r.Quantity < 0 {
		return errors.New("invalid quantity")
	}
	if r.Threshold < 0 {
		return errors.New("invalid threshold")
	}
	return nil
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

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

	records, err := parseCSV(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var imported int
	var errorsList []ProductValidationError

	for i, rec := range records {
		rowNum := i + 2 // header is row 1

		if err := validateRow(rec); err != nil {
			errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: %v", rowNum, err)})
			continue
		}

		existing, err := productRepo.GetByName(rec.Name)
		if err == nil && existing.ID != 0 {
			if mode == "skip" {
				errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: product '%s' already exists", rowNum, rec.Name)})
				continue
			}
			existing.Price = rec.Price
			existing.Quantity = rec.Quantity
			existing.Threshold = rec.Threshold
			existing.UpdatedAt = nowRFC3339()
			if _, err := productRepo.Update(existing); err != nil {
				errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: failed to update '%s'", rowNum, rec.Name)})
				continue
			}
			imported++
			continue
		}

		newProduct := models.Product{
			Name:      rec.Name,
			Price:     rec.Price,
			Quantity:  rec.Quantity,
			Threshold: rec.Threshold,
			CreatedAt: nowRFC3339(),
			UpdatedAt: nowRFC3339(),
		}
		if _, err := productRepo.Create(newProduct); err != nil {
			errorsList = append(errorsList, ProductValidationError{Description: fmt.Sprintf("row %d: %v", rowNum, err)})
			continue
		}
		imported++
	}

	err = writeJSON(w, http.StatusOK, ImportProductsResult{
		ImportedProductsCount: imported,
		Errors:                errorsList,
	})

	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
}
