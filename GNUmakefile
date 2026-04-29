default: fmt lint install generate

GO ?= go

build:
	$(GO) build -v ./...

install: build
	$(GO) install -v ./...

lint:
	golangci-lint run

generate:
	$(GO) -C tools generate ./...

fmt:
	gofmt -s -w -e .

test:
	$(GO) test -v -cover -timeout=120s -parallel=10 ./...

testacc: export TF_ACC=1
testacc:
	$(GO) test -v -cover -timeout 120m ./...

build-importer:
	$(GO) build -o bin/guardicore-importer ./cmd/importer/

.PHONY: fmt lint test testacc build install generate build-importer
