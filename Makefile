.PHONY: deps build test docker-up docker-down proto lint

deps:
	go mod tidy && go mod download

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -o bin/grpc-server ./cmd/grpc-server

test:
	go test ./tests/... -v -count=1

cover:
	go test ./tests/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Raport: coverage.html"

docker-up:
	docker compose up --build

docker-down:
	docker compose down

docker-clean:
	docker compose down -v --rmi local

proto:
	protoc --go_out=. --go-grpc_out=. proto/currency.proto

lint:
	golangci-lint run ./...
