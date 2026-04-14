# AGENTS Guide

This file is for coding agents working in `axiom-lambda-extension`.

## Project Summary

- Language: Go (`go 1.24.0`)
- Artifact: AWS Lambda extension binary packaged as a Lambda Layer
- Purpose: Receive Lambda Telemetry API events over local HTTP, transform/enrich events, and batch-ingest into Axiom.

## Repository Layout

- `main.go`: process entrypoint and orchestration.
- `extension/extension.go`: Lambda Extensions API client (`/2020-01-01/extension`) for register + next event.
- `telemetryapi/telemetry.go`: Lambda Telemetry API client (`/2022-07-01/telemetry`) for subscription.
- `server/server.go`: local HTTP ingestion endpoint for telemetry batches, event normalization/enrichment.
- `flusher/flusher.go`: event queue, flush policy, retry vs no-retry Axiom clients.
- `version/version.go`: extension version constant exposed in event metadata and user agent.
- `Makefile`: local build/test/package/publish helpers.
- `.github/workflows/ci.yaml`: CI, build matrix, and publish flows.

## Runtime Architecture

1. `main.Run()` creates an Axiom flusher client (`flusher.New()`).
2. Starts local HTTP server on port `8080` (`server.New(...).Run(ctx)`).
3. Registers with Lambda Extensions API (`extension.Client.Register`).
4. Subscribes to telemetry (`function` + `platform`) via Telemetry API.
5. Extension loop waits on `NextEvent` and flushes opportunistically.
6. `server` receives telemetry events, enriches with lambda/extension metadata, normalizes message/time fields, and queues events to flusher.
7. First `platform.runtimeDone` triggers a one-time signal back to `main` to flush after first invocation.
8. On shutdown or process exit, a final flush is attempted with retry enabled.

## Important Environment Variables

- `AWS_LAMBDA_RUNTIME_API`: runtime API host:port provided by Lambda.
- `AXIOM_TOKEN`: API token for ingestion.
- `AXIOM_DATASET`: dataset name for ingest target.
- `PANIC_ON_API_ERR`: if `true`, startup failures creating Axiom client return an error instead of continuing.

Read-only Lambda metadata is captured in `server` from:

- `AWS_LAMBDA_FUNCTION_NAME`
- `AWS_REGION`
- `AWS_LAMBDA_FUNCTION_VERSION`
- `AWS_LAMBDA_INITIALIZATION_TYPE`
- `AWS_LAMBDA_FUNCTION_MEMORY_SIZE`

## Local Development Commands

- Run tests:
  - `make test`
  - Optional explicit architecture: `GOARCH=amd64 make test` or `GOARCH=arm64 make test`
- Build layer binary:
  - `GOARCH=amd64 make build`
  - `GOARCH=arm64 make build`
- Package zip (`bin/extension.zip`):
  - `GOARCH=amd64 make package`
- Clean artifacts:
  - `make clean`

For local process-only debugging without Lambda registration/subscription, use:

- `go run . --development-mode`

## Linting and Formatting

The repo uses `golangci-lint` (config in `.golangci.yaml`).

Enabled linters include:

- `errcheck`, `govet` (with `shadow`), `staticcheck`, `gosec`, `ineffassign`, `unused`, and others.

Formatters enabled through golangci config:

- `gofmt` (simplify enabled)
- `goimports` with local prefix `github.com/axiomhq/axiom-lambda-extension`

Recommended validation before opening/updating PRs:

1. `golangci-lint run`
2. `make test`
3. `GOARCH=amd64 make build` (and `arm64` if touching build/runtime-sensitive code)

## CI Expectations

`ci.yaml` runs:

- `lint` job: `golangci-lint-action`
- `test` job: `make test` (after lint)
- `build` job: `make build` for `amd64` and `arm64`

Pushes to `main` in `axiomhq/*` also trigger development layer publishing.
Tags `v*` trigger production publish across AWS regions.

## Change Guidelines for Agents

- Keep the extension event loop in `main.go` non-blocking except where intentionally waiting for lifecycle signals.
- Preserve `runtimeDone` first-invocation behavior unless intentionally redesigning startup/flush semantics.
- Keep telemetry event transformations in `server/httpHandler` backwards-compatible.
- Use `flusher.SafelyUseAxiomClient` when interacting with flusher from call sites where client may be nil.
- If changing ingestion behavior, verify both retry and non-retry flush paths.
- If bumping extension version, update `version/version.go`.
- Do not commit build artifacts from `bin/`.

## Testing Notes

There are currently no `*_test.go` files in the repository. If you add logic-heavy changes, add focused unit tests in the relevant package and keep `make test` green.

## Typical Safe Workflow

1. Make scoped change in one package.
2. Run formatter/lint checks.
3. Run `make test`.
4. Build for target arch(es).
5. Update docs (`README.md` and/or this file) if behavior/config changed.
