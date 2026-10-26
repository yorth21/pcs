# ODD Task Tracker — Profile Scope Enhancements (Issues #11, #12, #13)

## Objective
Implement issues #11, #12, and #13:
- #11: Install `claude-pcs` launcher on PATH, isolate environment variables (`setlocal`/`endlocal`, `try/finally`, `exec env`), support `--launcher-dir` and `--launcher-args`.
- #12: Reuse main profile's MCP servers via generated `<targetDir>/shared-mcp.json` passed via `--mcp-config`, regenerating on `pcs sync`.
- #13: Bootstrap statusLine from main profile, support `--with-plugins`, login guidance, extend `pcs doctor --scope profile`, and document in README.

## Tasks
- [x] Task 1: Update `internal/installer/installer.go` and `internal/cli/cli.go` with launcher path isolation, `--launcher-dir`, `--launcher-args`, shared-mcp generation, statusLine copy, and `--with-plugins`/`--no-statusline`.
- [x] Task 2: Extend `internal/doctor/doctor.go` and `internal/cli/cli.go` to support `--scope profile` checks (launcher on PATH, shared-mcp, plugins, auth credentials existence).
- [x] Task 3: Add unit tests in `internal/installer/installer_test.go`, `internal/cli/cli_test.go`, and `internal/doctor/doctor_test.go`.
- [x] Task 4: Update `README.md` and `docs/architecture.md`.
- [x] Task 5: Run `go test -v -race ./...` and `go vet ./...`, build `./bin/pcs`, commit, push, and close issues #11, #12, #13.
