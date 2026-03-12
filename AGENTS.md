# AGENTS.md

Use simple language. Be direct. Do not pad answers.

## Project shape

- This repo is a Go CLI.
- News is the only supported area right now.
- Keep the command structure small and explicit. Do not add a large CLI framework unless there is a clear reason.

## Architecture rules

- Keep backend access in `internal/newsapi`.
- Keep HTML and terminal rendering logic in `internal/render`.
- Keep TUI code in `internal/tui`.
- Keep command parsing and output routing in `internal/commands`.

## Product rules

- `viola news` is the main entrypoint.
- Interactive mode should work in TTYs.
- Plain text and JSON modes must stay script-friendly.
- The article body source of truth is the detail API `text` field, not page scraping.

## Verification

Before closing work, run:

```bash
go test ./...
go build ./cmd/viola
```

If you change command behavior, also run at least one local smoke test with `./viola news ...`.

