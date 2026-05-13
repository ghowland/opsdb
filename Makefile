SHELL := /bin/sh

GO ?= go
GOFMT ?= gofmt
GOFLAGS ?=
LDFLAGS ?=
BUILD_DIR ?= bin

PKG_ALL := ./...
CMD_DIRS := \
	./tools/opsdb-api/cmd \
	./tools/opsdb-schema/cmd \
	./tools/importers/opsdb-import-aws/cmd \
	./tools/importers/opsdb-import-gcp/cmd \
	./tools/importers/opsdb_import_identity/cmd \
	./tools/importers/opsdb_import_k8s/cmd \
	./tools/importers/opsdb_import_monitoring/cmd \
	./tools/importers/opsdb_import_oncall/cmd \
	./tools/importers/opsdb_import_secrets/cmd \
	./tools/runners/change-set-executor/cmd \
	./tools/runners/emergency-review-monitor/cmd \
	./tools/runners/notification-runner/cmd \
	./tools/runners/reaper/cmd \
	./tools/runners/schema-executor/cmd

BINS := \
	opsdb-api \
	opsdb-schema \
	opsdb-import-aws \
	opsdb-import-gcp \
	opsdb-import-identity \
	opsdb-import-k8s \
	opsdb-import-monitoring \
	opsdb-import-oncall \
	opsdb-import-secrets \
	change-set-executor \
	emergency-review-monitor \
	notification-runner \
	reaper \
	schema-executor

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Common targets:"
	@echo "  make build          Build all binaries"
	@echo "  make build-one CMD=./tools/opsdb-api/cmd OUT=opsdb-api"
	@echo "  make run CMD=./tools/opsdb-api/cmd"
	@echo "  make test           Run unit tests"
	@echo "  make test-race      Run tests with race detector"
	@echo "  make test-cover     Run tests with coverage"
	@echo "  make fmt            Format Go code"
	@echo "  make vet            Run go vet"
	@echo "  make lint           Run fmt check + vet"
	@echo "  make tidy           Run go mod tidy"
	@echo "  make clean          Remove build artifacts"
	@echo "  make list           List buildable commands"

.PHONY: list
list:
	@printf '%s\n' $(CMD_DIRS)

.PHONY: dirs
dirs:
	@mkdir -p "$(BUILD_DIR)"

.PHONY: build
build: dirs \
	$(BUILD_DIR)/opsdb-api \
	$(BUILD_DIR)/opsdb-schema \
	$(BUILD_DIR)/opsdb-import-aws \
	$(BUILD_DIR)/opsdb-import-gcp \
	$(BUILD_DIR)/opsdb-import-identity \
	$(BUILD_DIR)/opsdb-import-k8s \
	$(BUILD_DIR)/opsdb-import-monitoring \
	$(BUILD_DIR)/opsdb-import-oncall \
	$(BUILD_DIR)/opsdb-import-secrets \
	$(BUILD_DIR)/change-set-executor \
	$(BUILD_DIR)/emergency-review-monitor \
	$(BUILD_DIR)/notification-runner \
	$(BUILD_DIR)/reaper \
	$(BUILD_DIR)/schema-executor

$(BUILD_DIR)/opsdb-api: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/opsdb-api/cmd

$(BUILD_DIR)/opsdb-schema: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/opsdb-schema/cmd

$(BUILD_DIR)/opsdb-import-aws: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb-import-aws/cmd

$(BUILD_DIR)/opsdb-import-gcp: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb-import-gcp/cmd

$(BUILD_DIR)/opsdb-import-identity: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_identity/cmd

$(BUILD_DIR)/opsdb-import-k8s: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_k8s/cmd

$(BUILD_DIR)/opsdb-import-monitoring: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_monitoring/cmd

$(BUILD_DIR)/opsdb-import-oncall: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_oncall/cmd

$(BUILD_DIR)/opsdb-import-secrets: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_secrets/cmd

$(BUILD_DIR)/change-set-executor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/change-set-executor/cmd

$(BUILD_DIR)/emergency-review-monitor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/emergency-review-monitor/cmd

$(BUILD_DIR)/notification-runner: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/notification-runner/cmd

$(BUILD_DIR)/reaper: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/reaper/cmd

$(BUILD_DIR)/schema-executor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/schema-executor/cmd

.PHONY: build-one
build-one: dirs
ifndef CMD
	$(error CMD is required, e.g. make build-one CMD=./tools/opsdb-api/cmd OUT=opsdb-api)
endif
	@out="$${OUT:-$$(basename "$(CMD)")}" ; \
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$(BUILD_DIR)/$$out" "$(CMD)"

.PHONY: run
run:
ifndef CMD
	$(error CMD is required, e.g. make run CMD=./tools/opsdb-api/cmd)
endif
	$(GO) run $(GOFLAGS) "$(CMD)"
	
.PHONY: test
test:
	$(GO) test $(GOFLAGS) $(PKG_ALL)

.PHONY: test-race
test-race:
	$(GO) test $(GOFLAGS) -race $(PKG_ALL)

.PHONY: test-cover
test-cover:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out -covermode=atomic $(PKG_ALL)
	$(GO) tool cover -func=coverage.out

.PHONY: bench
bench:
	$(GO) test $(GOFLAGS) -bench=. -benchmem $(PKG_ALL)

.PHONY: fmt
fmt:
	$(GOFMT) -w $$(find . -type f -name '*.go' \
		-not -path './vendor/*' \
		-not -path './bin/*')

.PHONY: fmt-check
fmt-check:
	@unformatted="$$(gofmt -l $$(find . -type f -name '*.go' \
		-not -path './vendor/*' \
		-not -path './bin/*'))" ; \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted Go files:" ; \
		echo "$$unformatted" ; \
		exit 1 ; \
	fi

.PHONY: vet
vet:
	$(GO) vet $(PKG_ALL)

.PHONY: lint
lint: fmt-check vet

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: mod-download
mod-download:
	$(GO) mod download

.PHONY: clean
clean:
	rm -rf "$(BUILD_DIR)" coverage.out

.PHONY: rebuild
rebuild: clean build
