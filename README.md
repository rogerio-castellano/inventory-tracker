# Inventory Tracker API

## Implemented features

- HTTP Server (main.go)
- Clean layering (internal/http, internal/repo)
- RESTful endpoints:

  - POST /products
  - GET /products
  - DELETE /products/{id}

- In-memory persistence with repo.ProductRepository
- Thorough handler tests (including malformed input)
- Auto-cleanup logic in tests with t.Cleanup
