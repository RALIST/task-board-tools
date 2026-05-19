GOLANGCI_LINT ?= golangci-lint

dev:
	cd gui && wails3 dev

lint-go:
	GOLANGCI_LINT="$(GOLANGCI_LINT)" ./scripts/lint-go.sh

.PHONY: dev lint-go
