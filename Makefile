# on Windows use Git Bash.
SHELL := bash

.PHONY: dev dev-client build test lint vuln generate sqlc-check client ci

dev:
	go run ./cmd/noted

dev-client:
	cd web && npm run dev

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o noted ./cmd/noted

test:
	go test -race ./...

lint:
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@2026.1 ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

generate:
	go tool sqlc generate

sqlc-check:
	go tool sqlc diff

client:
	cd web && npm run check && npm run build

ci: lint test sqlc-check vuln client
