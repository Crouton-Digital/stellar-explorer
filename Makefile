.PHONY: format format-check test check build

format:
	go run github.com/segmentio/golines@v0.13.0 --base-formatter=gofmt --max-len=100 -w cmd internal migrations docs
	cd web && npm run format

format-check:
	test -z "$$(gofmt -l cmd internal migrations docs/*.go)"
	cd web && npm run format:check

test:
	go test ./...
	cd web && npm test

check:
	go vet ./...
	cd web && npm run typecheck

build:
	go build -buildvcs=false ./cmd/...
	cd web && npm run build
