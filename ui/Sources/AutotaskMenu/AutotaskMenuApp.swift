import SwiftUI
import AppKit

@main
struct AutotaskMenuApp: App {
    @StateObject private var store = TaskStore()

    var body: some Scene {
        MenuBarExtra {
            MenuPanelView(store: store)
                .frame(width: 380, height: 520)
        } label: {
            MenuBarIcon(hasIssues: store.issueCount > 0)
        }
        .menuBarExtraStyle(.window)

        Window("Autotask", id: "dashboard") {
            DashboardView(store: store)
                .frame(minWidth: 760, minHeight: 520)
        }
        .defaultSize(width: 900, height: 620)
    }
}

struct MenuBarIcon: View {
    var hasIssues: Bool

    var body: some View {
        if let image = NSImage(named: "menubar-icon") {
            Image(nsImage: templateImage(image))
                .resizable()
                .scaledToFit()
                .frame(width: 18, height: 18)
                .overlay(alignment: .topTrailing) {
                    if hasIssues {
                        Circle()
                            .fill(Color.red)
                            .frame(width: 6, height: 6)
                            .offset(x: 3, y: -3)
                    }
                }
        } else {
            Image(systemName: hasIssues ? "exclamationmark.triangle" : "clock.badge.checkmark")
        }
    }

    private func templateImage(_ image: NSImage) -> NSImage {
        guard let copy = image.copy() as? NSImage else {
            image.isTemplate = true
            return image
        }
        copy.isTemplate = true
        return copy
    }
}

struct MenuPanelView: View {
    @ObservedObject var store: TaskStore
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            header

            if let error = store.errorMessage {
                ErrorView(message: error)
            }

            Divider()

            if store.tasks.isEmpty && store.isLoading {
                ProgressView("Loading tasks...")
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 8) {
                        ForEach(store.tasks.prefix(10)) { task in
                            MenuTaskRow(task: task)
                        }
                    }
                    .padding(.vertical, 2)
                }
            }

            Divider()

            HStack {
                Button("Refresh") {
                    Task { await store.refresh() }
                }
                .keyboardShortcut("r")

                Button("Open Dashboard") {
                    NSApp.activate(ignoringOtherApps: true)
                    openWindow(id: "dashboard")
                    Task { await store.refreshIfNeeded() }
                }

                Spacer()

                Button("Quit") {
                    NSApplication.shared.terminate(nil)
                }
                .keyboardShortcut("q")
            }
        }
        .padding(14)
        .task {
            await store.refreshIfNeeded()
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text("Autotask")
                    .font(.headline)
                Spacer()
                if store.isLoading {
                    ProgressView()
                        .scaleEffect(0.65)
                }
            }

            Text(summaryText)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .monospacedDigit()

            if let lastRefresh = store.lastRefresh {
                Text("Updated \(lastRefresh.formatted(date: .omitted, time: .shortened))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var summaryText: String {
        let count = store.state?.summary?.tasks ?? store.tasks.count
        let issues = store.issueCount
        let diff = store.diffCount
        let sync = diff == 0 ? "synced" : "\(diff) diff"
        return "\(count) tasks · \(issues) issues · \(sync)"
    }
}

struct MenuTaskRow: View {
    var task: TaskStatus

    var body: some View {
        HStack(spacing: 8) {
            StatusDot(status: task.status, enabled: task.enabled)
            VStack(alignment: .leading, spacing: 2) {
                HStack {
                    Text(task.name)
                        .font(.body)
                        .lineLimit(1)
                    Spacer()
                    RunMarksView(records: task.runs?.recent ?? [])
                    Text(displaySchedule(task.schedule))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
                Text(task.status ?? "unknown")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
        }
    }
}

struct DashboardView: View {
    @ObservedObject var store: TaskStore

    var body: some View {
        HStack(spacing: 0) {
            SidebarView(store: store)
                .frame(width: 280)

            Divider()

            DetailView(store: store)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .toolbar {
            ToolbarItem {
                Button("Refresh") {
                    Task { await store.refresh() }
                }
            }
        }
        .task {
            await store.refreshIfNeeded()
            if let selected = store.selectedTask {
                await store.loadDetail(for: selected.name)
            }
        }
    }
}

struct SidebarView: View {
    @ObservedObject var store: TaskStore

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Tasks")
                .font(.headline)
                .padding(.horizontal, 12)
                .padding(.top, 12)

            if let error = store.errorMessage {
                ErrorView(message: error)
                    .padding(.horizontal, 12)
            }

            List(selection: selectedBinding) {
                ForEach(store.tasks) { task in
                    VStack(alignment: .leading, spacing: 3) {
                        HStack {
                            StatusDot(status: task.status, enabled: task.enabled)
                            Text(task.name)
                                .lineLimit(1)
                            Spacer()
                            RunMarksView(records: task.runs?.recent ?? [])
                        }
                        Text(displaySchedule(task.schedule))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .tag(task.name)
                }
            }
            .listStyle(.sidebar)
        }
    }

    private var selectedBinding: Binding<String?> {
        Binding {
            store.selectedTask?.name
        } set: { name in
            guard let name, let task = store.tasks.first(where: { $0.name == name }) else {
                return
            }
            Task {
                await store.select(task)
            }
        }
    }
}

struct DetailView: View {
    @ObservedObject var store: TaskStore

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let selected = store.selectedTask {
                detailContent(selected)
            } else {
                VStack(spacing: 10) {
                    Image(systemName: "clock")
                        .font(.largeTitle)
                        .foregroundStyle(.secondary)
                    Text("No Task Selected")
                        .font(.headline)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
    }

    @ViewBuilder
    private func detailContent(_ selected: TaskStatus) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                HStack(alignment: .firstTextBaseline) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(selected.name)
                            .font(.title2)
                            .fontWeight(.semibold)
                        Text(selected.label)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }

                    Spacer()

                    Button("Reload Detail") {
                        Task { await store.loadDetail(for: selected.name) }
                    }
                }

                if store.isLoadingDetail {
                    ProgressView("Loading detail...")
                }

                statusGrid(selected)

                if let task = store.detail?.task {
                    detailGrid(task)
                }

                logsSection(selected)
            }
            .padding(20)
        }
    }

    private func statusGrid(_ selected: TaskStatus) -> some View {
        Grid(alignment: .leading, horizontalSpacing: 16, verticalSpacing: 8) {
            GridRow {
                FieldLabel("Status")
                Text(selected.status ?? "unknown")
            }
            GridRow {
                FieldLabel("Enabled")
                Text((selected.enabled ?? false) ? "yes" : "no")
            }
            GridRow {
                FieldLabel("Schedule")
                Text(displaySchedule(selected.schedule))
            }
            if let runs = selected.runs, let recent = runs.recent, !recent.isEmpty {
                GridRow {
                    FieldLabel("Recent")
                    RunMarksView(records: recent)
                }
                if let last = runs.last {
                    GridRow {
                        FieldLabel("Last Run")
                        Text("\(last.success ? "ok" : "failed") · exit \(last.exitCode) · \(durationText(last.durationMS))")
                    }
                }
            }
            if let path = selected.path {
                GridRow {
                    FieldLabel("Plist")
                    Text(compactHome(path))
                        .textSelection(.enabled)
                }
            }
        }
    }

    private func detailGrid(_ task: TaskConfig) -> some View {
        Grid(alignment: .leading, horizontalSpacing: 16, verticalSpacing: 8) {
            if let log = task.log {
                GridRow {
                    FieldLabel("Log")
                    Text(compactHome(log))
                        .textSelection(.enabled)
                }
            }
            if let tags = task.tags, !tags.isEmpty {
                GridRow {
                    FieldLabel("Tags")
                    Text(tags.joined(separator: ", "))
                }
            }
            if let notes = task.notes, !notes.isEmpty {
                GridRow {
                    FieldLabel("Notes")
                    Text(notes)
                        .textSelection(.enabled)
                }
            }
            if let command = task.command, !command.isEmpty {
                GridRow(alignment: .top) {
                    FieldLabel("Command")
                    Text(command.map(compactHome).joined(separator: " "))
                        .textSelection(.enabled)
                        .font(.system(.body, design: .monospaced))
                }
            }
        }
    }

    private func logsSection(_ selected: TaskStatus) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Logs")
                    .font(.headline)
                Spacer()
                Button(store.isLoadingLogs ? "Loading..." : "Load Logs") {
                    Task { await store.loadLogs(for: selected.name) }
                }
                .disabled(store.isLoadingLogs)
            }

            if store.logs.isEmpty {
                Text("Logs are loaded only when requested.")
                    .foregroundStyle(.secondary)
            } else {
                ScrollView {
                    Text(store.logs)
                        .font(.system(.caption, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(10)
                }
                .frame(minHeight: 160, maxHeight: 260)
                .background(Color(nsColor: .textBackgroundColor))
                .clipShape(RoundedRectangle(cornerRadius: 6))
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(Color(nsColor: .separatorColor))
                )
            }
        }
    }
}

struct RunMarksView: View {
    var records: [RunRecord]

    var body: some View {
        if records.isEmpty {
            Text("-")
                .font(.caption)
                .foregroundStyle(.secondary)
        } else {
            HStack(spacing: 4) {
                ForEach(records.prefix(5)) { record in
                    RoundedRectangle(cornerRadius: 2)
                        .fill(record.success ? Color.green : Color.red)
                        .frame(width: 12, height: 6)
                        .accessibilityLabel(record.success ? "ok" : "failed")
                }
            }
        }
    }
}

struct StatusDot: View {
    var status: String?
    var enabled: Bool?

    var body: some View {
        Circle()
            .fill(color)
            .frame(width: 8, height: 8)
            .accessibilityLabel(status ?? "unknown")
    }

    private var color: Color {
        if enabled == false {
            return .secondary
        }
        let value = (status ?? "").lowercased()
        if value.contains("running") {
            return .green
        }
        if value.contains("invalid") || value.contains("error") || value.contains("not-installed") {
            return .red
        }
        if value.contains("loaded") || value.contains("started") {
            return .blue
        }
        return .secondary
    }
}

struct FieldLabel: View {
    var text: String

    init(_ text: String) {
        self.text = text
    }

    var body: some View {
        Text(text)
            .foregroundStyle(.secondary)
            .frame(width: 76, alignment: .leading)
    }
}

struct ErrorView: View {
    var message: String

    var body: some View {
        Text(message)
            .font(.caption)
            .foregroundStyle(.red)
            .textSelection(.enabled)
            .fixedSize(horizontal: false, vertical: true)
    }
}

func displaySchedule(_ schedule: String?) -> String {
    guard let schedule, !schedule.isEmpty else {
        return "-"
    }
    return schedule
}

func compactHome(_ value: String) -> String {
    let home = NSHomeDirectory()
    guard !home.isEmpty else {
        return value
    }
    return value.replacingOccurrences(of: home, with: "~")
}

func durationText(_ ms: Int64?) -> String {
    guard let ms else {
        return "-"
    }
    if ms >= 60_000 {
        return String(format: "%.1fm", Double(ms) / 60_000.0)
    }
    if ms >= 1_000 {
        return String(format: "%.1fs", Double(ms) / 1_000.0)
    }
    return "\(ms)ms"
}
