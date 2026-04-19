# Autotask Menu

Read-only SwiftUI menubar for `autotask`.

## Build

```sh
make build
```

The app bundle is written to:

```sh
ui/build/AutotaskMenu.app
```

Run it with:

```sh
make run
```

When installed through Homebrew, launch the menubar app with:

```sh
autotask-menu
```

## Behavior

- Calls `/opt/homebrew/bin/autotask` by default.
- Set `AUTOTASK_CLI=/path/to/autotask` to override the CLI path.
- Reads state with `autotask ui-state --json`.
- Reads details with `autotask show <name> --json`.
- Reads logs with `autotask logs <name> -n 80`.
- Shows recent run results recorded by `autotask exec`.
- Does not edit tasks, run tasks, enable/disable tasks, or sync launchd.

The app refreshes on open and on manual refresh. It does not poll in the background.
