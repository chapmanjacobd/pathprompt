BINARY := pathprompt
CMD := ./cmd/pathprompt

.PHONY: build test install

all: fmt lint build test install

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

install:
	go install $(CMD)

fmt:
	gofmt -s -w -e .
	-goimports -w -e .
	-gofumpt -w .
	-gci write .
	go fix ./...

lint:
	golangci-lint run --fix ./...
