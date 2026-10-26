.PHONY: build run test tidy docker

build:
	go build -o bin/device-mock ./cmd/device-mock

run:
	go run ./cmd/device-mock -config configs/devices.json

test:
	go test ./...

tidy:
	go mod tidy

docker:
	docker build -t device-mock:latest .
