import Foundation

@MainActor
final class TaskStore: ObservableObject {
    @Published var state: UIState?
    @Published var selectedTask: TaskStatus?
    @Published var detail: TaskDetailResponse?
    @Published var logs: String = ""
    @Published var isLoading = false
    @Published var isLoadingDetail = false
    @Published var isLoadingLogs = false
    @Published var errorMessage: String?
    @Published var lastRefresh: Date?

    private let cli = AutotaskCLI()

    var tasks: [TaskStatus] {
        state?.tasks ?? []
    }

    var issues: [DoctorIssue] {
        state?.issues ?? []
    }

    var diffCount: Int {
        state?.summary?.diff ?? state?.diff?.count ?? 0
    }

    var issueCount: Int {
        state?.summary?.issues ?? issues.count
    }

    func refreshIfNeeded() async {
        if state == nil {
            await refresh()
        }
    }

    func refresh() async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            let next = try await cli.uiState()
            state = next
            lastRefresh = Date()
            if let selectedTask, let replacement = next.tasks.first(where: { $0.name == selectedTask.name }) {
                self.selectedTask = replacement
            } else if selectedTask == nil {
                self.selectedTask = next.tasks.first
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func select(_ task: TaskStatus) async {
        selectedTask = task
        logs = ""
        await loadDetail(for: task.name)
    }

    func loadDetail(for name: String) async {
        isLoadingDetail = true
        errorMessage = nil
        defer { isLoadingDetail = false }
        do {
            detail = try await cli.show(name)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func loadLogs(for name: String) async {
        isLoadingLogs = true
        errorMessage = nil
        defer { isLoadingLogs = false }
        do {
            logs = try await cli.logs(name)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
