VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

-include .env.mk
DEPLOY_HOST ?= user@host
DEPLOY_PATH ?= ~/.local/bin/pingolin

build:
	go build -ldflags "$(LDFLAGS)" -o pingolin .

install: build
	cp pingolin /usr/local/bin/

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f pingolin pingolin-linux

deploy: test
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o pingolin-linux .
	scp pingolin-linux $(DEPLOY_HOST):/tmp/pingolin-new
	ssh $(DEPLOY_HOST) "\
		sudo systemctl stop pingolin-web.service 2>/dev/null; \
		sudo systemctl stop pingolin.service 2>/dev/null; \
		mv /tmp/pingolin-new $(DEPLOY_PATH) && \
		chmod +x $(DEPLOY_PATH) && \
		sudo systemctl start pingolin.service && \
		sudo systemctl start pingolin-web.service && \
		$(DEPLOY_PATH) version"
	rm -f pingolin-linux

.PHONY: build install test lint clean deploy
