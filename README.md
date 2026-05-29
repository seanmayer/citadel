# gwtui

`gwtui` is a Bubble Tea terminal UI for browsing Git worktrees, opening the selected worktree in your editor or terminal, and running Git commands inside it.

## Features

- Detects the current repository root when launched inside a Git repository
- Loads worktrees via `git worktree list --porcelain`
- Shows per-worktree status badges for remote tracking, merge state, and dirty state
- Scrolls through worktrees with keyboard navigation
- Shows selected worktree details, including branch, commit, upstream, ahead/behind counts, merge state, and dirty state
- Opens the selected worktree in a configurable editor command, defaulting to VS Code via `code .`
- Opens the selected worktree in a configurable terminal command
- Shows a per-worktree action list with shortcuts for terminal commands, editor launch, staging, commit, branch creation, and delete
- Lets you create a branch for detached or branchless worktrees
- Lets you delete a worktree and its local branch with a confirmation prompt
- Opens a command mode to run raw Git commands like `git status`, `git fetch`, or `git log --oneline -5`
- Stages all files in the selected worktree with `git add .`
- Opens a commit message prompt and runs `git commit -m "..."` in the selected worktree
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
- `o`: open the selected worktree in the configured editor
- `t`: open the selected worktree in a new terminal
- `a`: stage all files in the selected worktree with `git add .`
- `c`: open commit mode and write the commit message for `git commit -m "..."`
- `b`: create a branch for the selected detached or branchless worktree
- `d`: delete the selected worktree and confirm with `y`
- `enter`: open command mode or run the command
- `space` / `enter`: continue from the command output view back to the worktree list
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
  open_editor: "o"
  open_terminal: "t"
  refresh: "r"
  fetch_refresh: "R"
  quit: "q"
  help: "?"

editor:
  command: "code"
  args:
    - "."

terminal:
  command: "open"
  args:
    - "-a"
    - "Terminal"
    - "{path}"

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
- `keybindings.open_editor`: key used to launch the selected worktree in the editor
- `keybindings.open_terminal`: key used to launch the selected worktree in a new terminal
- `editor.command`: executable used to launch the editor, such as `code`, `zed`, or `open`
- `editor.args`: optional args passed before launch; the selected worktree is the command working directory, and `{path}` expands to its full path
- `terminal.command`: executable used to launch a terminal app, such as `open`, `wezterm`, or `wt`
- `terminal.args`: optional args passed before launch; the selected worktree is the command working directory, and `{path}` expands to its full path

The default terminal launcher is platform-specific. The example above shows the macOS default.

See [config.example.yaml](config.example.yaml) for a ready-to-copy example.

## Development

Run formatting and tests:

```bash
gofmt -w ./cmd ./internal
go test ./...
```
