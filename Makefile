SHELL := /bin/bash

SERVER_DIR := server
WEBAPP_DIR := webapp
E2E_DIR := e2e
BUILD_DIR := build
PLUGIN_ARCHIVE := $(BUILD_DIR)/plugins/mm-oidc.tar.gz
PLUGIN_ID := com.mm.oidc
PACKAGE_ROOT := $(BUILD_DIR)/package
PLUGIN_STAGING := $(PACKAGE_ROOT)/$(PLUGIN_ID)
SERVER_DIST_DIR := $(SERVER_DIR)/dist
WEBAPP_DIST_DIR := $(WEBAPP_DIR)/dist
SERVER_BINARY := plugin-linux-amd64
YARN := corepack yarn

.PHONY: server-test server-build webapp-install webapp-build webapp-test webapp-lint e2e-install e2e-test package dev-up dev-down dev-logs clean-plugin

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

e2e-install:
	cd $(E2E_DIR) && $(YARN) install --inline-builds
	cd $(E2E_DIR) && $(YARN) playwright install chromium

e2e-test:
	./scripts/e2e-test.sh

package: server-build webapp-build
	rm -rf $(PLUGIN_STAGING)
	mkdir -p $(PLUGIN_STAGING)/server/dist $(PLUGIN_STAGING)/webapp/dist
	cp plugin.json $(PLUGIN_STAGING)/
	cp $(SERVER_DIST_DIR)/$(SERVER_BINARY) $(PLUGIN_STAGING)/server/dist/
	cp -R $(WEBAPP_DIST_DIR)/. $(PLUGIN_STAGING)/webapp/dist/
	mkdir -p $(BUILD_DIR)/plugins
	tar -czvf $(PLUGIN_ARCHIVE) -C $(PACKAGE_ROOT) $(PLUGIN_ID)

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
