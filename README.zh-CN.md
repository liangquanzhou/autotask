# autotask

`autotask` 是一个面向 macOS 的个人自动任务管理 CLI。它的目标不是替代 launchd、crontab 或 Homebrew services，而是把你自己配置的任务集中登记、检查和同步，避免任务散落在不同地方之后忘记来源、重复建设。

## 安装

```sh
brew tap liangquanzhou/tap
brew install autotask
```

已经安装过时可以升级：

```sh
brew update
brew upgrade liangquanzhou/tap/autotask
```

安装后 CLI 命令是：

```sh
autotask version
```

menubar 只读界面可以用这个命令启动：

```sh
autotask-menu
```

## 核心模型

`autotask` 把 `~/.config/autotask/tasks.yaml` 当作你的个人任务登记表。

- `autotask list` 只显示登记表里的任务。
- `autotask scan` 扫描系统现状，包括 crontab、launchd 和 brew services。
- `autotask doctor` 检查重复任务、无效 plist、登记表与系统状态不一致等问题。
- `autotask diff` 对比登记表和 `~/Library/LaunchAgents` 里的实际 plist。
- `autotask sync --apply` 按登记表生成并加载用户级 LaunchAgent。

默认建议是：你自己的长期自动任务进入 `tasks.yaml`，再由 `autotask sync --apply` 生成 launchd plist。系统、第三方软件、Homebrew services 仍由它们自己的工具管理，`autotask` 只做扫描和提醒。

## 常用命令

```sh
autotask scan --personal
autotask scan --personal --verbose
autotask doctor --personal
autotask list
autotask list --group
autotask list --verbose
autotask status
autotask show <name>
autotask diff
autotask sync --apply
autotask import --apply --refresh
autotask run <name>
autotask logs <name>
autotask enable <name>
autotask disable <name>
autotask fix-cron-dupes --apply
autotask ui-state --json
```

## list、scan、doctor 的区别

`autotask list` 不是“列出系统全部后台任务”，而是列出 `~/.config/autotask/tasks.yaml` 中登记的任务。它更像一个个人资产台账。

默认的 `autotask list` 只显示概览字段：任务名、调度时间、类型和 label。长命令不会出现在默认列表里，避免把表格撑乱。

- `autotask list --group`：按 Daemon / Daily / Weekly / Monthly / Interval 分组查看。
- `autotask list --verbose`：用卡片形式展示所有登记任务的完整信息。
- `autotask show <name>`：只查看一个任务的完整信息，包括 command、plist、日志路径、备注等。

`autotask scan` 才是扫描当前机器上真实存在的任务来源：

- 当前用户的 `crontab`
- `~/Library/LaunchAgents`
- `/Library/LaunchAgents`
- `/Library/LaunchDaemons`
- `brew services list`

`autotask scan --personal` 会尽量过滤出像是你自己配置的任务，例如 label 符合个人前缀，或者命令路径指向你的项目、配置目录。这个过滤是启发式的，所以它适合快速排查，不适合当作最终配置来源。

默认的 `autotask scan` 会按来源分组，只显示状态、调度时间和 label。需要查看命令和 plist 路径时使用 `autotask scan --verbose`。

`autotask doctor --personal` 会在扫描结果基础上检查问题，比如：

- 同一个脚本同时存在于 crontab 和 launchd
- plist XML 无效
- 登记表里的任务没有对应的 launchd plist
- 登记表里的命令文件不存在
- 登记表和 launchd 里的内容不一致

## brew services 怎么处理

Homebrew services 不会自动进入 `autotask list`。

这是刻意的：`brew services` 已经有自己的状态、启动和停止模型，适合继续用 Homebrew 管理。`autotask scan` 会读取它们，方便你看全局状态和发现重复，但默认不会把它们纳入 `tasks.yaml`。

如果某个 Homebrew service 实际上是你自己写的长期任务，也可以手动登记到 `tasks.yaml`，但一般不建议这么做。更清晰的做法是：

- 数据库、nginx、ollama 这类服务继续用 `brew services`。
- 你自己的脚本、日报、同步、监控任务放进 `autotask`。

## crontab 怎么处理

crontab 也不会自动进入 `autotask list`，除非你把它转换成登记表里的任务。

当前推荐路线是：个人长期任务优先迁到 launchd，由 `autotask` 管理。原因是 launchd 是 macOS 原生机制，能被 `launchctl` 查询和加载，也更适合后续做 SwiftUI 管理界面。

`autotask fix-cron-dupes --apply` 可以删除那些已经在 `tasks.yaml` 中登记、并且也有 launchd 版本的重复 crontab 行。

## 运行历史

`autotask exec <name>` 是给 launchd 使用的执行包装器。它的含义是：由 `autotask` 代为执行登记表里的原始 command，并记录这次运行是否成功、开始/结束时间、退出码和耗时。

记录会写到：

```text
~/.config/autotask/runs/<name>.jsonl
```

查看最近运行记录：

```sh
autotask runs data-asset-sync
autotask runs data-asset-sync --json
```

注意：已有 LaunchAgent 只有在重新执行 `autotask sync --apply` 生成 wrapper plist 之后，才会开始积累结构化运行历史。daemon / keepalive 这类常驻任务不会被 wrapper，因为它们不是“一次运行一次退出”的任务。

## 后续 UI

后续 SwiftUI 不需要自己解析 plist 或 crontab。它应该只调用：

```sh
autotask ui-state --json
```

这个命令会返回适合 UI 使用的结构，包括：

- `tasks`：登记任务和当前状态
- `diff`：需要处理的差异
- `issues`：doctor 检查出来的问题
- `summary`：任务数、问题数、差异数

这样 Go CLI 负责系统交互和同步逻辑，SwiftUI 只负责展示、确认和触发操作。

## 菜单栏 UI

只读 SwiftUI 菜单栏应用放在 `ui/` 目录：

```sh
cd ui
make build
make run
```

第一版只读取状态，不执行写操作：

- `autotask ui-state --json`
- `autotask show <name> --json`
- `autotask logs <name> -n 80`

它不会编辑任务、运行任务、启停任务或执行 `sync --apply`。这些操作继续交给 CLI 或 agent。
