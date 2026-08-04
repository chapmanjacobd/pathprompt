GO ?= go
BINARY := pathprompt
CMD := ./cmd/pathprompt

.PHONY: build test install

build:
	$(GO) build -o $(BINARY) $(CMD)

test:
	$(GO) test ./...

install:
	$(GO) install $(CMD)
