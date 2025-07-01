# Inventory Tracker API

## Implemented features

- HTTP Server (main.go)
- RESTful endpoints:

  - POST /products
  - GET /products
  - DELETE /products/{id}

- In-memory persistence with repo.ProductRepository
- Handler tests (including malformed input)
- Auto-cleanup logic in tests with t.Cleanup
