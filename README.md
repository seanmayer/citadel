# gwtui

`gwtui` is a Bubble Tea terminal UI for browsing Git worktrees and running Git commands inside the selected worktree.

## Features

- Detects the current repository root when launched inside a Git repository
- Loads worktrees via `git worktree list --porcelain`
- Scrolls through worktrees with keyboard navigation
- Shows selected worktree details, including branch, commit, current-worktree status, and optional dirty state
- Lets you create a branch for detached or branchless worktrees
- Opens a command mode to run raw Git commands like `git status`, `git fetch`, or `git log --oneline -5`
- Displays command output directly inside the TUI

## Install

```bash
go install github.com/seanmayer/citadel/cmd/gwtui@latest
```

## Run

From anywhere inside a Git repository:

```bash
go run ./cmd/gwtui
```

Or run the installed binary:

```bash
gwtui
```

## Keybindings

- `up` / `k`: move selection up
- `down` / `j`: move selection down
- `b`: create a branch for the selected detached or branchless worktree
- `enter`: open command mode or run the command
- `r`: refresh worktrees
- `esc`: leave command mode or output view
- `?`: show help
- `q` / `ctrl+c`: quit

## Configuration

By default `gwtui` loads config from:

```text
~/.config/gwtui/config.yaml
```

If the file is missing, the app falls back to sane defaults.

Example config:

```yaml
default_command: "git status"

keybindings:
  refresh: "r"
  quit: "q"
  help: "?"

ui:
  show_dirty_status: true
  show_commit_hash: true
  show_branch: true
```

See [config.example.yaml](/Users/sean/.codex/worktrees/690c/citadel/config.example.yaml) for a ready-to-copy example.

## Development

Run formatting and tests:

```bash
gofmt -w ./cmd ./internal
go test ./...
```
