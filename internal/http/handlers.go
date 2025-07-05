package http

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

var userRepo repo.UserRepository

type ProductRequest struct {
	Id       int     `json:"id,omitempty"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type ProductResponse struct {
	Id       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
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

func CreateProductHandler(w http.ResponseWriter, r *http.Request) {
	var req ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	validationErrors := validateProduct(req)
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": validationErrors,
		})
		return
	}

	product := models.Product{
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	created, err := productRepo.Create(product)
	if err != nil {
		http.Error(w, "could not create product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:       created.ID,
		Name:     created.Name,
		Price:    created.Price,
		Quantity: created.Quantity,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	products, err := productRepo.GetAll()
	if err != nil {
		http.Error(w, "could not fetch products", http.StatusInternalServerError)
		return
	}
	response := make([]ProductResponse, len(products))
	for i, p := range products {
		response[i] = ProductResponse{
			Id:       p.ID,
			Name:     p.Name,
			Price:    p.Price,
			Quantity: p.Quantity,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetProductByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	product, err := productRepo.GetByID(id)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not fetch product", http.StatusInternalServerError)
		return
	}
	resp := ProductResponse{
		Id:       product.ID,
		Name:     product.Name,
		Price:    product.Price,
		Quantity: product.Quantity,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func DeleteProductHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id") // Use chi to get the path parameter
	if idStr == "" {
		http.Error(w, "product ID is required", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}
	if err := productRepo.Delete(id); err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not delete product", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func UpdateProductHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	var req ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	validationErrors := validateProduct(req)
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": validationErrors,
		})
		return
	}

	product := models.Product{
		ID:        id,
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	updated, err := productRepo.Update(product)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not update product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:       updated.ID,
		Name:     updated.Name,
		Price:    updated.Price,
		Quantity: updated.Quantity,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func FilterProductsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	name := query.Get("name")

	var (
		minPrice, maxPrice            *float64
		minQty, maxQty, offset, limit *int
	)

	if v := query.Get("minPrice"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			minPrice = &val
		}
	}
	if v := query.Get("maxPrice"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			maxPrice = &val
		}
	}
	if v := query.Get("minQty"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			minQty = &val
		}
	}
	if v := query.Get("maxQty"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			maxQty = &val
		}
	}

	if v := query.Get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			offset = &val
		}
	}

	if v := query.Get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			limit = &val
		}
	}

	if v := query.Get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err != nil || val <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		} else {
			limit = &val
		}
	}

	if v := query.Get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err != nil || val < 0 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		} else {
			offset = &val
		}
	}

	products, totalCount, err := productRepo.Filter(name, minPrice, maxPrice, minQty, maxQty, offset, limit)
	if err != nil {
		http.Error(w, "could not filter products", http.StatusInternalServerError)
		return
	}

	var response ProductsSearchResult
	for _, p := range products {
		response.Data = append(response.Data, ProductResponse{
			Id:       p.ID,
			Name:     p.Name,
			Price:    p.Price,
			Quantity: p.Quantity,
		})
	}

	response.Meta.TotalCount = totalCount

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func AdjustQuantityHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	var req QuantityAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	product, err := productRepo.GetByID(id)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch product", http.StatusInternalServerError)
		return
	}

	newQty := product.Quantity + req.Delta
	if newQty < 0 {
		http.Error(w, "quantity cannot be negative", http.StatusConflict)
		return
	}

	product.Quantity = newQty
	product.UpdatedAt = time.Now().Format(time.RFC3339)

	updated, err := productRepo.Update(product)
	if err != nil {
		http.Error(w, "could not update product", http.StatusInternalServerError)
		return
	}

	err = movementRepo.Log(id, req.Delta)
	if err != nil {
		// Log the error but do not return it to the user
		// This allows the product update to succeed even if logging fails
		log.Printf("could not log movement for product %d, delta %d: %v", id, req.Delta, err)
	}

	resp := ProductResponse{
		Id:       updated.ID,
		Name:     updated.Name,
		Price:    updated.Price,
		Quantity: updated.Quantity,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func GetMovementsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	// Validate the product ID
	_, err = productRepo.GetByID(id)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
	}

	sinceStr := r.URL.Query().Get("since")
	untilStr := r.URL.Query().Get("until")

	// Reverse the substitution from + for space in the date parameters, otherwise
	// time.Parse will fail with an error.
	// This is necessary because URL query parameters replace + with a space.
	// Example: 2025-07-03T17:44:03+02:00 becomes 2025-07-03T17:44:03 02:00 on r.URL.Query().Get()
	if len(sinceStr) == len(time.RFC3339) && sinceStr[len(sinceStr)-6] == ' ' {
		sinceStr = sinceStr[:len(sinceStr)-6] + "+" + sinceStr[len(sinceStr)-5:]
	}
	if len(untilStr) == len(time.RFC3339) && untilStr[len(untilStr)-6] == ' ' {
		untilStr = untilStr[:len(untilStr)-6] + "+" + untilStr[len(untilStr)-5:]
	}

	var since, until *time.Time
	if sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &ts
		} else {
			log.Printf("could not parse since date %s: %v", sinceStr, err)
			http.Error(w, "invalid since date format", http.StatusBadRequest)
			return
		}
	}
	if untilStr != "" {
		if ts, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &ts
		} else {
			log.Printf("could not parse until date %s: %v", untilStr, err)
			http.Error(w, "invalid until date format", http.StatusBadRequest)
			return
		}
	}

	var limit, offset *int

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = &v
		} else {
			log.Printf("could not parse limit %s: %v", limitStr, err)
			http.Error(w, "invalid limit format", http.StatusBadRequest)
			return
		}
	}

	if limit != nil && *limit <= 0 {
		log.Printf("invalid limit %d, must be greater than zero", *limit)
		http.Error(w, "limit must be greater than zero", http.StatusBadRequest)
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			offset = &v
		} else {
			log.Printf("could not parse offset %s: %v", offsetStr, err)
			http.Error(w, "invalid offset format", http.StatusBadRequest)
			return
		}
	}

	if offset != nil && *offset < 0 {
		log.Printf("invalid offset %d, must be zero or positive", *offset)
		http.Error(w, "offset must be zero or positive", http.StatusBadRequest)
		return
	}

	movements, total, err := movementRepo.GetByProductID(id, since, until, limit, offset)

	if err != nil {
		log.Printf("could not retrieve movements for product %d: %v", id, err)
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}
	response := MovementsSearchResult{
		Data: make([]MovementResponse, len(movements)),
		Meta: Meta{TotalCount: total},
	}

	for i, m := range movements {
		response.Data[i] = MovementResponse{
			ID:        m.ID,
			ProductID: m.ProductID,
			Delta:     m.Delta,
			CreatedAt: m.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func validateProduct(p ProductRequest) map[string]string {
	errs := make(map[string]string)
	if strings.TrimSpace(p.Name) == "" {
		errs["name"] = "Name is required"
	}
	if p.Price <= 0 {
		errs["price"] = "Price must be greater than zero"
	}
	if p.Quantity < 0 {
		errs["quantity"] = "Quantity cannot be negative"
	}
	return errs
}

func ExportMovementsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		http.Error(w, "format must be 'csv' or 'json'", http.StatusBadRequest)
		return
	}

	sinceStr := r.URL.Query().Get("since")
	untilStr := r.URL.Query().Get("until")

	if len(sinceStr) == len(time.RFC3339) && sinceStr[len(sinceStr)-6] == ' ' {
		sinceStr = sinceStr[:len(sinceStr)-6] + "+" + sinceStr[len(sinceStr)-5:]
	}
	if len(untilStr) == len(time.RFC3339) && untilStr[len(untilStr)-6] == ' ' {
		untilStr = untilStr[:len(untilStr)-6] + "+" + untilStr[len(untilStr)-5:]
	}

	var since, until *time.Time
	if sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &ts
		}
	}
	if untilStr != "" {
		if ts, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &ts
		}
	}

	movements, _, err := movementRepo.GetByProductID(id, since, until, nil, nil)
	if err != nil {
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="movements.json"`)
		json.NewEncoder(w).Encode(movements)

	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="movements.csv"`)

		csvWriter := csv.NewWriter(w)
		_ = csvWriter.Write([]string{"id", "product_id", "delta", "c"})
		for _, m := range movements {
			_ = csvWriter.Write([]string{
				strconv.Itoa(m.ID),
				strconv.Itoa(m.ProductID),
				strconv.Itoa(m.Delta),
				m.CreatedAt,
			})
		}
		csvWriter.Flush()
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	user, err := userRepo.GetByUsername(creds.Username)

	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, "could not generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func SetUserRepo(r repo.UserRepository) {
	userRepo = r
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	if len(creds.Username) < 3 || len(creds.Password) < 6 {
		http.Error(w, "username or password too short", http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user := models.User{
		Username:     creds.Username,
		PasswordHash: string(hashed),
	}

	_, err = userRepo.CreateUser(user)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, "username already exists", http.StatusConflict)
		} else {
			http.Error(w, "failed to register user", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "user registered"})
}
