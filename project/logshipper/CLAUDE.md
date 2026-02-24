# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Layout

This directory is an IntelliJ workspace. The actual Go source code lives at `../collector` (module name: `collector`, Go 1.24).

```
../collector/
  cmd/collector/main.go     # entry point
  internal/
    app/         # wires sources, transforms, sinks into a Pipeline
    config/      # YAML config struct, loader, validator
    event/       # Event struct (the common data unit)
    parse/       # auto-detects and parses plain text / JSON / ECS JSON / metrics
    pipeline/    # Pipeline runtime: runs goroutines, connects channels
    sources/     # stdin, file (tail), docker
    transform/   # remap-lite: add fields + case conversion
    sinks/       # stdout
  config.yml     # example config (uses .env vars)
  .env.example   # required env vars
```

## Commands (run from `../collector`)

```bash
# Build
go build -o collector ./cmd/collector/main.go

# Run (config file is required)
./collector -c config.yml

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/sources/...

# Run a single test
go test ./internal/transform/... -run TestRemapTransform

# Docker build
docker build -t logshipper .
```

## Architecture

The pipeline is a linear chain of Go channels:

```
Sources (goroutines) → sourceChan → [parse stage] → parsedChan → [transform] → sinkChan → Sink
```

All three stages (`Source`, `Transformer`, `Sink`) share the same `context.Context` for graceful shutdown via `SIGINT`/`SIGTERM`.

**Event** (`internal/event/event.go`) is the single data type that flows through every stage. It holds `Timestamp`, `Source`, `Service`, `Type` (log or metric), `Level`, `Message`, `Attrs`, and metric fields.

**Parsing** (`internal/parse/`) is done automatically in `pipeline.go` after the source stage — no config needed. It detects plain text, generic JSON, ECS JSON (Elastic Common Schema), or metric payloads and populates Event fields accordingly.

**Config** is YAML with three top-level sections (`sources`, `transforms`, `sinks`). Environment variables in config values (e.g., `${SERVICE_NAME}`) are expanded at load time. The `.env` file is loaded automatically; copy `.env.example` to `.env` before running.

**Current limitations (enforced in `app.go`):** only 1 transform and 1 sink are supported at a time.

## Adding a New Component

- **Source**: implement `pipeline.Source` interface (`Run(ctx, out chan<- event.Event) error`), add a `case` in `app.go`'s `buildPipeline`, and add a `SourceConfig` field in `config.go` if new YAML fields are needed.
- **Transform**: implement `pipeline.Transformer` (`Run(ctx, in, out chan<- event.Event) error`), add a `case` in `buildPipeline`.
- **Sink**: implement `pipeline.Sink` (`Run(ctx, in <-chan event.Event) error`), add a `case` in `buildPipeline` and `SinkConfig` fields if needed.
