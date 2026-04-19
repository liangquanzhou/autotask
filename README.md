# autotask

`autotask` is a lightweight CLI for keeping personal macOS automation tasks visible and auditable.

[中文说明](README.zh-CN.md)

## Install

```sh
brew tap liangquanzhou/tap
brew install autotask
```

Or build from source:

```sh
make install
```

The long-term model is:

- personal scripts and schedules live in `~/.config/autotask/tasks.yaml`
- generated LaunchAgents live under `~/.config/autotask/generated/`
- logs live under `~/.config/autotask/logs/`
- Homebrew services stay managed by `brew services`
- system and third-party launchd jobs are scanned read-only

## Commands

```sh
autotask scan [--json] [--personal] [--verbose]
autotask doctor [--json] [--personal] [--verbose]
autotask list [--json] [--group] [--verbose]
autotask status [name] [--json]
autotask show <name> [--json]
autotask ui-state --json
autotask diff [--json]
autotask sync [--apply] [--json]
autotask import [--apply] [--refresh] [--json]
autotask run <name>
autotask logs <name> [-n N]
autotask edit [name]
autotask fix-cron-dupes [--apply] [--json]
autotask enable <name>
autotask disable <name>
autotask init
autotask version
```

`scan` reads:

- current user's `crontab`
- `~/Library/LaunchAgents`
- `/Library/LaunchAgents`
- `/Library/LaunchDaemons`
- `brew services list`

Default human-readable output is intentionally compact. Use `list --group` to group registered tasks by schedule, `list --verbose` for full task cards, and `show <name>` for one task's details. Use `scan --verbose` when you need command and plist paths for discovered tasks.

`doctor` checks:

- invalid plist files
- duplicate scheduled commands across schedulers
- missing registry file
- registered tasks whose labels or executables are missing
- missing log directories

`diff` compares the registry with user LaunchAgents. It treats plist XML formatting and key order as irrelevant and compares the launchd fields that `autotask` manages.

`ui-state --json` returns one object with `status`, `diff`, and `issues`. This is the intended API for a future Swift menu bar app.

`sync` is dry-run by default. Use `sync --apply` to write generated plists to both:

- `~/.config/autotask/generated/<label>.plist`
- `~/Library/LaunchAgents/<label>.plist`

When a task is enabled, `sync --apply` uses modern launchd commands:

```sh
launchctl bootout gui/$UID/<label>
launchctl bootstrap gui/$UID ~/Library/LaunchAgents/<label>.plist
```

`import --apply --refresh` updates `tasks.yaml` from existing personal LaunchAgents while preserving command arguments, working directory, environment variables, log paths, and common launchd options.

`fix-cron-dupes` removes crontab entries whose command is already registered as a launchd task in `tasks.yaml`. It is dry-run by default.

## Development

The project `Makefile` unexports `GOROOT` so a stale shell `GOROOT` cannot conflict with the `go` binary found on `PATH`:

```sh
make test
make install
autotask doctor --personal
```

The future Swift UI should call this CLI with `--json` and keep all launchd/plist logic in the Go binary.

## Menubar UI

A read-only SwiftUI menubar app lives in `ui/`.

```sh
cd ui
make build
make run
```

It reads from the CLI only:

- `autotask ui-state --json`
- `autotask show <name> --json`
- `autotask logs <name> -n 80`

It intentionally does not edit tasks, run tasks, enable/disable tasks, or sync launchd.

## Personal Matching

`scan --personal` treats a LaunchAgent as personal when its label matches one of:

```sh
AUTOTASK_PERSONAL_PREFIXES="local.,autotask.,me."
```

or when the command/path points into the current user's project/config directories. Override the prefixes with a comma-separated environment variable if your labels use a different namespace.
