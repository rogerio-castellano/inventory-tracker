# Build stage
FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o inventory-api ./api/main.go

# Runtime stage
FROM scratch
COPY --from=builder /app/inventory-api /inventory-api
EXPOSE 8080
ENTRYPOINT ["/inventory-api"]
