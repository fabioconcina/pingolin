VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o pingolin .

install: build
	cp pingolin /usr/local/bin/

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f pingolin

.PHONY: build install test lint clean
