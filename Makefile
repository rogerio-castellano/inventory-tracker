APP_NAME = inventory-api
IMAGE_NAME = inventory-tracker

build:
	go build -o $(APP_NAME) ./api/main.go

run:
	go run ./api/main.go

test:
	go test ./...

docker-build:
	docker build -t $(IMAGE_NAME) .

docker-run:
	docker run -p 8080:8080 --env DATABASE_URL=$(DATABASE_URL) $(IMAGE_NAME)
