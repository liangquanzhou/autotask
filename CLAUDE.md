# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目定位

`autotask` 是一个 macOS 个人任务自动化 CLI，目标是把散落在 crontab / LaunchAgents / LaunchDaemons / brew services 里的调度任务统一成**可见、可审计、可 diff、可同步**的状态。Source of truth 是 `~/.config/autotask/tasks.yaml`。

## 常用命令

```sh
make build            # go build ./...
make test             # go test ./...
make install          # go build -o ~/.local/bin/autotask .
go test -run TestBuildUIState_MergesRuntimeFailures ./...   # 跑单个测试

cd ui && make build   # 构建 SwiftUI menubar（arm64 macos13+）
cd ui && make run     # 构建后 open AutotaskMenu.app
```

`Makefile` 里 `unexport GOROOT` 是故意的——防止 shell 里遗留的 GOROOT 和 PATH 上的 go 不匹配。

## 架构

### 单文件 Go CLI + 只读 Swift menubar

- `main.go`（~2700 行）是全部 CLI 逻辑，没有拆 package
- `ui/`（~850 行 Swift）是只读 menubar，通过 `autotask ui-state --json` / `show <name> --json` / `logs <name>` 调 CLI。它**不写状态**（不改 yaml、不 enable/disable、不 sync）
- Swift 那边通过环境变量 `AUTOTASK_CLI` 可覆盖 CLI 路径，默认 `/opt/homebrew/bin/autotask`

### 配置与产物目录（都在 `configDir()` 下）

`configDir()` = `$XDG_CONFIG_HOME/autotask` 或 `~/.config/autotask`。`AUTOTASK_CONFIG` 可直接覆盖 `tasks.yaml` 路径。

| 子目录 | 内容 | 写入方 |
|---|---|---|
| `tasks.yaml` | Source of truth（`Task[]`） | 用户 / `import` / `edit` |
| `generated/<label>.plist` | `sync --apply` 生成的 plist 副本 | `writeTaskPlist` |
| `logs/<name>.log` | stdout/stderr | launchd |
| `runs/<name>.jsonl` | 每次运行的 `RunRecord` | `autotask exec` |

受管的 plist 同时也会写到 `~/Library/LaunchAgents/<label>.plist`（被 launchd 实际加载的位置）。

### exec wrapper 机制（关键）

调度任务的 launchd plist 里 `ProgramArguments` **不是**原始命令，而是 `autotask exec <name>`。由 `autotask exec` 读 yaml → fork 真命令 → 写 `RunRecord{exit_code, duration_ms, success}` 到 `runs/<name>.jsonl`。

- 决定是否 wrap：`shouldWrapTaskExecution`（daemon / KeepAlive 任务不 wrap，因为要直接保持驻留）
- 没有经过 `sync --apply` 重新生成的旧 plist 走不到 wrapper，所以也没有 run 记录——`import --apply --refresh` + `sync --apply` 是激活 run 历史的方式

### diff 语义

`taskSemanticallyEqual` + `comparableTask` 只比 launchd 实际使用的字段（命令、调度、env、log path 等），**忽略 plist XML 格式和 key 顺序**。不要引入 XML 字节级比较。

### ui-state JSON 合约（给 Swift）

`buildUIState`（`main.go:444`）是 Swift UI 的唯一入口。返回：

- `tasks`（= `status`，保留别名供向后兼容）：每个 `StatusRow` 含 `Runs RunInfo`，里面的 `Last *RunRecord` 用于判定 runtime 失败
- `diff`：过滤掉 `noop` 后的 actions
- `issues`：合并 `diff.Issues`（doctor/config drift 级别）**和** runtime 失败（最近 7 天内、且任务 enabled、由 `runtimeFailureIssue` 生成 `code=runtime_failure, level=warn`）
- `summary`：counts 同步 issues / tasks / diff / actions

改 JSON 结构时**必须**保证 Swift `TaskStore` / `Models.swift` 里 `UIState` 解码不挂——新字段加 optional，旧字段删前搜一下 `ui/Sources/`。

### 扫描源（`scanAll`）

- `crontab -l`
- `~/Library/LaunchAgents`、`/Library/LaunchAgents`、`/Library/LaunchDaemons`
- `brew services list`

`--personal` 过滤走 `looksPersonal`：label 前缀匹配（环境变量 `AUTOTASK_PERSONAL_PREFIXES`，默认 `local.,autotask.,me.`）**或**命令路径指向用户 home。

## 版本号

`const appVersion` 写死在 `main.go:24`。发版需要在这里改，并同步 README / brew formula。

## 测试约定

测试集中在 `main_test.go`（单 package）。关键测试：

- `TestRenderLaunchdPlistWrapsScheduledTasks` — 确保调度任务被 exec wrapper 包住
- `TestRenderLaunchdPlistEscapesShellOperators` — plist XML 转义（`&&` → `&amp;&amp;`）
- `TestBuildUIState_MergesRuntimeFailures` — ui-state issues 包含 runtime 失败（enabled + 7 天窗口 + 解析失败兜底）
- `TestBuildUIStateHasTaskAliasAndActionableDiff` — ui-state 的 `tasks` 别名和 `diff` noop 过滤

加新测试时：如果构造 `StatusRow` 且涉及 runtime 判定，记得同时设置 `Enabled: true` 和 `Runs.Last.EndedAt`（RFC3339），否则会被 `runtimeFailureIssue` 的早 return 跳过。

## 发布

通过 Homebrew tap `liangquanzhou/tap` 分发（见 README）。menubar app 通过 `autotask-menu` 命令启动（tap formula 负责软链）。
