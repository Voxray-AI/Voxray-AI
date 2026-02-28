# voila-go – build and run
BINARY_NAME := voila
MAIN_PKG := ./cmd/voila

.PHONY: build build-voice run run-voice clean test tidy proto swagger lint lint-fix evals

# proto: generate Go from wire_frames.proto (requires protoc and protoc-gen-go)
proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/frames/proto/wire/wire_frames.proto

build:
	go build -o $(BINARY_NAME) $(MAIN_PKG)

# build-voice: build with CGO so Opus encoder is included (required for WebRTC TTS). Needs gcc in PATH.
build-voice:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) $(MAIN_PKG)

run:
	go run $(MAIN_PKG)

# run-voice: run with CGO so WebRTC TTS works. E.g. make run-voice ARGS="-config config.json"
run-voice:
	CGO_ENABLED=1 go run $(MAIN_PKG) $(ARGS)

clean:
	-go clean
	-rm -f $(BINARY_NAME) $(BINARY_NAME).exe

test:
	go test ./...

tidy:
	go mod tidy

# swagger: regenerate API docs (requires: go install github.com/swaggo/swag/cmd/swag@latest)
swagger:
	swag init -g cmd/voila/main.go --parseDependency --parseInternal

# lint: run pre-commit checks (gofmt + go vet)
lint:
	@./scripts/pre-commit.sh

# lint-fix: fix formatting and optional golangci-lint
lint-fix:
	@./scripts/fix-lint.sh

# evals: run eval scenarios (default config and voila config)
evals:
	go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json
