# voila-go – build and run
BINARY_NAME := voila
MAIN_PKG := ./cmd/voila

.PHONY: build run clean test tidy proto

# proto: generate Go from pkg/frames/proto/frames.proto (requires protoc and protoc-gen-go)
proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/frames/proto/frames.proto

build:
	go build -o $(BINARY_NAME) $(MAIN_PKG)

run:
	go run $(MAIN_PKG)

clean:
	-go clean
	-rm -f $(BINARY_NAME) $(BINARY_NAME).exe

test:
	go test ./...

tidy:
	go mod tidy
