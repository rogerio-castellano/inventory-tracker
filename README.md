# 🧮 Inventory Tracker API

A full-featured backend service for managing products, stock levels, movement logs, alerts, and metrics — built in Go with PostgreSQL.

---

## 🚀 Features

- ✅ Product CRUD with validation
- 🔄 Atomic stock adjustments with history
- 📊 Admin dashboard metrics
- 🔔 Low stock alerts
- 🗂️ Product filtering + pagination
- 📥 Batch CSV import (with update/skip modes)
- 📤 Movement export (CSV/JSON)
- 🧑 User auth with JWT
- 🔐 Role-Based Access Control (RBAC) with roles & permissions
- 🚦 API rate limiting using Redis-based token bucket with per-user and role-specific quotas
- 🛡️ Ban & session revocation stored in Redis with TTL
- 📘 OpenAPI docs (`/swagger`)
- 📊 Prometheus `/metrics` endpoint for monitoring (**planned**)
- 🛡️ Ban & session revocation system
- 📧 Email alerts via SMTP
- 🧪 Full test coverage

---

## 🛠 Tech Stack

| Layer         | Tech                                                      |
| ------------- | --------------------------------------------------------- |
| Language      | Go 1.24.4                                                 |
| Router        | [Chi](https://github.com/go-chi/chi)                      |
| Database      | PostgreSQL                                                |
| Migrations    | [Soda](https://gobuffalo.io/documentation/database/soda/) |
| Docs          | [Swaggo](https://github.com/swaggo/swag)                  |
| Auth          | JWT                                                       |
| RBAC          | File-based roles & permissions                            |
| Rate Limiting | Redis-backed token bucket with role-based quotas          |
| Monitoring    | Prometheus + Grafana                                      |
| Container     | Docker, Docker Compose                                    |
| Tests         | `go test`, Dockerized                                     |

---

## 🧰 Getting started

### ▶️ Run Locally

```bash
make run
```

### 🐳 With Docker Compose

```bash
docker-compose up --build
```

### 🔗 API Documentation

- Swagger UI: [/swagger](http://localhost:8080/swagger)
- OpenAPI JSON: [/swagger/doc.json](http://localhost:8080/swagger/doc.json)

### 🧪 Testing

Run tests locally:

```bash
make test
```

Run tests in Docker:

```bash
make test-docker
```

### 🧾 CSV Import Format

```csv
name,price,quantity,threshold
Mouse,25.99,10,2
Keyboard,45.00,5,1
Monitor,199.99,2,1
```

Upload with:

```bash
curl -X POST http://localhost:8080/products/import \
  -F "file=@products.csv"
```

Use `?mode=update` to overwrite existing products.

### 🔐 Authentication

Use `/register` or `/login` to get a JWT token.

Then send it via:

```http
Authorization: Bearer <your-token>
```

### 📊 Admin Dashboard

Query high-level metrics:

```http
GET /metrics/dashboard
```

Returns product count, low stock alerts, most moved item, average prices, etc.

### 📁 Project Structure

```plaintext
api/                 # Entry point
internal/
  http/              # Handlers and routes
  repo/              # Repositories and interfaces
  db/                # Postgres connector
docs/                # Swagger files (generated)
```

### 📦 Build

```bash
make build           # local
make docker-build    # container
```

### 📜 License

MIT — free to use, modify, and share.
