import Foundation

struct UIState: Decodable {
    var version: String
    var config: String?
    var tasks: [TaskStatus]
    var diff: [DiffAction]?
    var issues: [DoctorIssue]?
    var summary: Summary?

    struct Summary: Decodable {
        var tasks: Int?
        var diff: Int?
        var issues: Int?
        var actions: Int?
    }
}

struct TaskStatus: Decodable, Identifiable, Hashable {
    var id: String { name }
    var name: String
    var label: String
    var kind: String?
    var enabled: Bool?
    var status: String?
    var schedule: String?
    var command: String?
    var path: String?
    var runs: RunInfo?
}

struct RunInfo: Decodable, Hashable {
    var recent: [RunRecord]?
    var last: RunRecord?
    var successStreak: Int?
    var failureCount: Int?

    enum CodingKeys: String, CodingKey {
        case recent
        case last
        case successStreak = "success_streak"
        case failureCount = "failure_count"
    }
}

struct RunRecord: Decodable, Identifiable, Hashable {
    var id: String { "\(startedAt)-\(exitCode)" }
    var task: String?
    var startedAt: String
    var endedAt: String?
    var exitCode: Int
    var durationMS: Int64?
    var success: Bool

    enum CodingKeys: String, CodingKey {
        case task
        case startedAt = "started_at"
        case endedAt = "ended_at"
        case exitCode = "exit_code"
        case durationMS = "duration_ms"
        case success
    }
}

struct DiffAction: Decodable, Hashable {
    var action: String?
    var task: String?
    var label: String?
    var path: String?
    var reason: String?
}

struct DoctorIssue: Decodable, Identifiable, Hashable {
    var id: String { [level, code, message, ref].compactMap { $0 }.joined(separator: "|") }
    var level: String?
    var code: String?
    var message: String?
    var ref: String?
}

struct TaskDetailResponse: Decodable {
    var task: TaskConfig
    var status: TaskStatus?
}

struct TaskConfig: Decodable, Hashable {
    var name: String
    var label: String
    var title: String?
    var kind: String?
    var enabled: Bool?
    var schedule: Schedule?
    var command: [String]?
    var workingDirectory: String?
    var log: String?
    var stdout: String?
    var stderr: String?
    var tags: [String]?
    var notes: String?

    enum CodingKeys: String, CodingKey {
        case name
        case label
        case title
        case kind
        case enabled
        case schedule
        case command
        case workingDirectory = "working_directory"
        case log
        case stdout
        case stderr
        case tags
        case notes
    }
}

struct Schedule: Decodable, Hashable {
    var type: String?
    var cron: String?
    var everySeconds: Int?
    var minute: Int?
    var hour: Int?
    var day: Int?
    var weekday: Int?
    var month: Int?

    enum CodingKeys: String, CodingKey {
        case type
        case cron
        case everySeconds = "every_seconds"
        case minute
        case hour
        case day
        case weekday
        case month
    }
}
