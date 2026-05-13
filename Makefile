SHELL := /bin/sh

GO ?= go
GOFMT ?= gofmt
GOFLAGS ?=
LDFLAGS ?=
BUILD_DIR ?= bin

PKG_ALL := ./...
CMD_DIRS := \
	./tools/opsdb_api/cmd \
	./tools/opsdb_schema/cmd \
	./tools/importers/opsdb_import_aws/cmd \
	./tools/importers/opsdb_import_gcp/cmd \
	./tools/importers/opsdb_import_identity/cmd \
	./tools/importers/opsdb_import_k8s/cmd \
	./tools/importers/opsdb_import_monitoring/cmd \
	./tools/importers/opsdb_import_oncall/cmd \
	./tools/importers/opsdb_import_secrets/cmd \
	./tools/runners/change_set_executor/cmd \
	./tools/runners/emergency_review_monitor/cmd \
	./tools/runners/notification_runner/cmd \
	./tools/runners/reaper/cmd \
	./tools/runners/schema_executor/cmd

BINS := \
	opsdb_api \
	opsdb_schema \
	opsdb_import_aws \
	opsdb_import_gcp \
	opsdb_import_identity \
	opsdb_import_k8s \
	opsdb_import_monitoring \
	opsdb_import_oncall \
	opsdb_import_secrets \
	change_set_executor \
	emergency_review_monitor \
	notification_runner \
	reaper \
	schema_executor

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Common targets:"
	@echo "  make build          Build all binaries"
	@echo "  make build-one CMD=./tools/opsdb_api/cmd OUT=opsdb_api"
	@echo "  make run CMD=./tools/opsdb_api/cmd"
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
	$(BUILD_DIR)/opsdb_api \
	$(BUILD_DIR)/opsdb_schema \
	$(BUILD_DIR)/opsdb_import_aws \
	$(BUILD_DIR)/opsdb_import_gcp \
	$(BUILD_DIR)/opsdb_import_identity \
	$(BUILD_DIR)/opsdb_import_k8s \
	$(BUILD_DIR)/opsdb_import_monitoring \
	$(BUILD_DIR)/opsdb_import_oncall \
	$(BUILD_DIR)/opsdb_import_secrets \
	$(BUILD_DIR)/change_set_executor \
	$(BUILD_DIR)/emergency_review_monitor \
	$(BUILD_DIR)/notification_runner \
	$(BUILD_DIR)/reaper \
	$(BUILD_DIR)/schema_executor

$(BUILD_DIR)/opsdb_api: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/opsdb_api/cmd

$(BUILD_DIR)/opsdb_schema: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/opsdb_schema/cmd

$(BUILD_DIR)/opsdb_import_aws: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_aws/cmd

$(BUILD_DIR)/opsdb_import_gcp: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_gcp/cmd

$(BUILD_DIR)/opsdb_import_identity: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_identity/cmd

$(BUILD_DIR)/opsdb_import_k8s: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_k8s/cmd

$(BUILD_DIR)/opsdb_import_monitoring: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_monitoring/cmd

$(BUILD_DIR)/opsdb_import_oncall: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_oncall/cmd

$(BUILD_DIR)/opsdb_import_secrets: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/importers/opsdb_import_secrets/cmd

$(BUILD_DIR)/change_set_executor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/change_set_executor/cmd

$(BUILD_DIR)/emergency_review_monitor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/emergency_review_monitor/cmd

$(BUILD_DIR)/notification_runner: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/notification_runner/cmd

$(BUILD_DIR)/reaper: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/reaper/cmd

$(BUILD_DIR)/schema_executor: dirs
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./tools/runners/schema_executor/cmd

.PHONY: build-one
build-one: dirs
ifndef CMD
	$(error CMD is required, e.g. make build-one CMD=./tools/opsdb_api/cmd OUT=opsdb_api)
endif
	@out="$${OUT:-$$(basename "$(CMD)")}" ; \
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$(BUILD_DIR)/$$out" "$(CMD)"

.PHONY: run
run:
ifndef CMD
	$(error CMD is required, e.g. make run CMD=./tools/opsdb_api/cmd)
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
