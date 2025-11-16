# Repository Guidelines

## Project Structure & Module Organization

Source files live at the repo root under the `github.com/koizuka/scraper` module; `session.go`, `chrome.go`, and `unified_*.go` define the public API while `*_test.go` files beside them exercise the behavior. HTML fixtures for form and replay tests are under `test/`, with Chrome-specific recordings in `test_chrome_replay/` and legacy cases in `test_replay/`. Keep reusable mocks in `test_helpers.go`. Documentation such as `README.md`, `UNIFIED_API_MIGRATION_GUIDE.md`, and `UNMARSHAL_REFERENCE.md` explains expected flows—update these when adding new entry points or protocols.

## Build, Test, and Development Commands

Use Go 1.16+ as declared in `go.mod`. Common commands:

- `go test ./...` – run the entire suite, including Chrome replay tests that only touch fixtures.
- `GO_TEST_FLAGS='-run ChromeReplay' go test ./...` – focus on the replay-specific cases without touching unrelated packages.
- `go vet ./...` – static analysis; run before sending a PR.
- `gofmt -w $(git ls-files '*.go')` – enforce canonical formatting; `goimports` is acceptable if you prefer it to manage imports simultaneously.

## Coding Style & Naming Conventions

Follow idiomatic Go style: tabs for indentation, camel-case for exported identifiers (e.g., `ChromeTimeoutError` in `error.go`) and lowercase package-level helpers. Keep packages small; avoid introducing new ones unless the API surface demands it. Place constants near their usage and prefer constructor helpers such as `scraper.NewSession`. Log via the console logger interfaces provided in `logger.go` to keep test output predictable.

## Testing Guidelines

Write table-driven tests with names like `TestComponent_Scenario` to match the existing suite (`chrome_replay_simple_test.go`, `form_test.go`, etc.). Use fixtures from `test/` and add new `.html/.meta` pairs there when exploring new DOM layouts. Exercise Chrome-dependent flows through the replay harness to keep CI deterministic; actual browser automation belongs in targeted integration tests guarded by build tags if needed. Aim to cover new branches and any custom errors, and keep helper assertions in `test_helpers.go` for reuse.

## Commit & Pull Request Guidelines

Recent history shows imperative, descriptive commit subjects with optional scopes, e.g., `fix: stabilize CI tests by improving Chrome startup reliability (#80)` or `Capture HTML before Chrome operations for timeout debugging`. Mirror that tone, include context (why, not just what), and reference GitHub issues in parentheses when relevant. For pull requests, include: summary of behavior change, reproduction steps, test evidence (`go test ./...` output or screenshots for Chrome flows), and any manual setup (e.g., required cookies). Call out breaking API changes clearly and update the relevant docs before requesting review.

## Security & Configuration Tips

Do not commit session data or cookies; `session.NewSession("name", logger)` writes to `name/cookie`, so add custom directories to `.gitignore` when testing. When invoking the Chrome path, prefer configuring behavior through `USE_CHROME`, `Headless`, and timeout fields as shown in `UNIFIED_API_MIGRATION_GUIDE.md`, instead of hard-coding values. Strip personal identifiers from HTML fixtures under `test/` and `chromeUserData/` before pushing.
