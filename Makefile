SHELL := /bin/bash

SERVER_DIR := server
WEBAPP_DIR := webapp
BUILD_DIR := build
PLUGIN_ARCHIVE := $(BUILD_DIR)/plugins/mm-oidc.tar.gz
SERVER_BINARY := plugin-linux-amd64
YARN := corepack yarn

.PHONY: server-test server-build webapp-install webapp-build webapp-test webapp-lint package dev-up dev-down dev-logs clean-plugin

server-test:
	cd $(SERVER_DIR) && go test ./...

server-build:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=amd64 go build -o dist/$(SERVER_BINARY) ./...

webapp-install:
	cd $(WEBAPP_DIR) && $(YARN) install --inline-builds

webapp-build: webapp-install
	cd $(WEBAPP_DIR) && $(YARN) build

webapp-test: webapp-install
	cd $(WEBAPP_DIR) && $(YARN) test

webapp-lint: webapp-install
	cd $(WEBAPP_DIR) && $(YARN) lint

package: server-build webapp-build
	mkdir -p $(BUILD_DIR)/plugins
	tar -czvf $(PLUGIN_ARCHIVE) plugin.json $(SERVER_DIR)/dist/$(SERVER_BINARY) $(WEBAPP_DIR)/dist/main.js

clean-plugin:
	rm -f $(PLUGIN_ARCHIVE)
	rm -rf $(SERVER_DIR)/dist/$(SERVER_BINARY)
	rm -rf $(WEBAPP_DIR)/dist

dev-up:
	./scripts/dev-up.sh

dev-down:
	./scripts/dev-down.sh

SERVICES ?=
dev-logs:
	./scripts/dev-logs.sh $(SERVICES)
