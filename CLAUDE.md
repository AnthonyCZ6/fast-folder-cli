# fast-folder-cli

Fast folder search for Windows: a Go 1.26 CLI with an interactive Bubble Tea v2 TUI.
Module `github.com/AnthonyCZ6/fast-folder-cli`.

## Commands (mirror `.github/workflows/release.yml`)
- Format: `gofmt -l .` must print nothing
- Vet: `go vet ./...`
- Test: `go test -count=1 ./...` (CI adds `-race` on Linux)
- Build: `go build ./...`; releases use `-trimpath -ldflags "-s -w -X main.version=<version>"` for windows/amd64 and windows/arm64

## Layout
- `main.go`: entry point; `version` is set at build time via `-ldflags`
- `internal/`: `cli`, `search`, `tui`, `launch`, `pathutil`, `period`, `humanize`
- `shell/`: `fcd` wrappers (`fcd.cmd`, `fcd.ps1`)
- `install.ps1`, `installer/fast-folder-cli.iss`: PowerShell installer and Inno Setup wizard

## Conventions
- Code comments, README and CI step names are in Spanish
- Commit messages: English Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `ci:`, `build:`)
