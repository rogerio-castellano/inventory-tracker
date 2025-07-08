APP_NAME = inventory-api
IMAGE_NAME = inventory-tracker
PWD = $(shell pwd)

build:
	go build -o $(APP_NAME) ./api/main.go

run:
	go run ./api/main.go

test:
	go test ./...

docker-build:
	docker build -t $(IMAGE_NAME) .

docker-run:
	docker-compose up --build

docker-test:
	MSYS_NO_PATHCONV=1 docker run --rm \
 	-v "$(PWD)":/app \
 	-w /app \
 	-e DATABASE_URL=postgres://postgres:example@host.docker.internal:5432/inventory?sslmode=disable \
 	golang:1.24 \
 	go test ./...
