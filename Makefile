BINARY    := collector
BUILD_DIR := ./bin
BENCH_DIR := ./bench
PROF_DIR  := $(BENCH_DIR)/profiles
GO        := go

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "build | run | run-tui | test | test-race | bench | bench-profile | lint | clean"

.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/collector/main.go

.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY) -c config.yml

.PHONY: run-tui
run-tui: build
	$(BUILD_DIR)/$(BINARY) -c config-tui.yml -tui

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-race
test-race:
	$(GO) test -race ./...

.PHONY: bench
bench:
	$(GO) test -bench=. -benchmem -count=3 $(BENCH_DIR)/...
	$(GO) test -v -run 'TestDatasetSanity|TestAnomalyEval|TestTimeToDiagnose' $(BENCH_DIR)/...

.PHONY: bench-profile
bench-profile:
	@mkdir -p $(PROF_DIR)
	$(GO) test -bench=. -count=1 -cpuprofile=$(PROF_DIR)/cpu.prof $(BENCH_DIR)/...
	$(GO) test -bench='BenchmarkMemory' -count=1 -memprofile=$(PROF_DIR)/mem.prof $(BENCH_DIR)/...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(PROF_DIR)
	$(GO) clean -testcache