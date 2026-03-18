# PR Fixes Plan

## 1. Wire RCAService (`internal/infrastructure/config/container.go`)
- Update `createInvestigationComponents` to accept `aiAdapter port.AIProvider`.
- Instantiate `service.NewRCAService(aiAdapter)` and call `investigationUseCase.SetRCAService()`.
- Update `NewContainer` to pass `aiAdapter`.

## 2. Fix HistoryFile Paths
- `internal/infrastructure/config/config.go`: Change `~/.github.com/anthony-bible/code-agent-demo-history` to `~/.code-agent-demo-history`.
- `internal/infrastructure/config/config_test.go`: Update expected paths.
- `internal/infrastructure/config/container_test.go`: Update expected path.

## 3. Fix Binary Name in `serve.go`
- `cmd/cli/cmd/serve.go`: Change `github.com/anthony-bible/code-agent-demo serve` to `code-agent-demo serve` in lines 32-33.

## 4. Fix Doubled Module Paths in `README.md`
- Fix URLs on lines 5, 267, 284, 908, 911 to single paths.

## 5. Use slog in `alert_handler.go`
- Replace `fmt.Fprintf(os.Stderr, ...)` with `slog.Info`, `slog.Error`, `slog.Warn`. Add `"log/slog"` import.

## 6. Improve Test Coverage in `rca_service_test.go`
- Add table-driven or sub-tests for `RCAService.Correlate` edge cases:
  - Error from AI provider
  - Markdown-fenced JSON payload
  - Invalid JSON / failure to validate
  - Unparseable content
  - Empty findings
