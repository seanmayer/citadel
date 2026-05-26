# gwtui

`gwtui` is a Bubble Tea terminal UI for browsing Git worktrees and running Git commands inside the selected worktree.

## Features

- Detects the current repository root when launched inside a Git repository
- Loads worktrees via `git worktree list --porcelain`
- Shows per-worktree status badges for remote tracking, merge state, and dirty state
- Scrolls through worktrees with keyboard navigation
- Shows selected worktree details, including branch, commit, upstream, ahead/behind counts, merge state, and dirty state
- Lets you create a branch for detached or branchless worktrees
- Lets you delete a worktree and its local branch with a confirmation prompt
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
- `d`: delete the selected worktree and confirm with `y`
- `enter`: open command mode or run the command
- `r`: refresh worktrees and branch status, optionally fetching first if `git.fetch_on_refresh` is enabled
- `R`: always run `git fetch --prune`, then refresh worktrees and branch status
- `esc`: leave command mode or output view
- `?`: show help
- `q` / `ctrl+c`: quit

## Status Badges

Worktree rows can include compact badges such as:

- `clean`
- `dirty`
- `local`
- `↑N`
- `↓N`
- `merged`
- `not merged`
- `error`

Merge status is checked against the configured `git.base_branch`, which defaults to `origin/main`.

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
  fetch_refresh: "R"
  quit: "q"
  help: "?"

git:
  base_branch: "origin/main"
  fetch_on_refresh: false
  show_remote_status: true
  show_merge_status: true
  show_dirty_status: true

ui:
  show_commit_hash: true
  show_branch: true
```

Config notes:

- `git.base_branch`: branch or ref used for merge checks, such as `origin/main`
- `git.fetch_on_refresh`: if `true`, `r` runs `git fetch --prune` before status refresh
- `git.show_remote_status`: show `local` and `↑N` / `↓N` badges
- `git.show_merge_status`: show `merged` / `not merged` badges
- `git.show_dirty_status`: show `clean` / `dirty` badges

See [config.example.yaml](config.example.yaml) for a ready-to-copy example.

## Development

Run formatting and tests:

```bash
gofmt -w ./cmd ./internal
go test ./...
```
