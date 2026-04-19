import Foundation

enum AutotaskError: LocalizedError {
    case cliNotFound
    case commandFailed(command: String, code: Int32, stderr: String)
    case invalidOutput(command: String)

    var errorDescription: String? {
        switch self {
        case .cliNotFound:
            return "autotask CLI not found. Install it with Homebrew or set AUTOTASK_CLI."
        case let .commandFailed(command, code, stderr):
            let detail = stderr.trimmingCharacters(in: .whitespacesAndNewlines)
            return detail.isEmpty ? "\(command) failed with exit code \(code)." : "\(command) failed with exit code \(code): \(detail)"
        case let .invalidOutput(command):
            return "\(command) returned output that could not be decoded."
        }
    }
}

actor AutotaskCLI {
    private let decoder = JSONDecoder()

    func uiState() async throws -> UIState {
        let data = try await run(["ui-state", "--json"])
        return try decode(UIState.self, from: data, command: "autotask ui-state --json")
    }

    func show(_ name: String) async throws -> TaskDetailResponse {
        let data = try await run(["show", name, "--json"])
        return try decode(TaskDetailResponse.self, from: data, command: "autotask show \(name) --json")
    }

    func logs(_ name: String, lines: Int = 80) async throws -> String {
        let data = try await run(["logs", name, "-n", String(lines)])
        return String(data: data, encoding: .utf8) ?? ""
    }

    private func decode<T: Decodable>(_ type: T.Type, from data: Data, command: String) throws -> T {
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw AutotaskError.invalidOutput(command: command)
        }
    }

    private func run(_ arguments: [String]) async throws -> Data {
        let executable = try locateExecutable()
        return try await Task.detached(priority: .userInitiated) {
            let process = Process()
            process.executableURL = executable
            process.arguments = arguments
            process.environment = Self.environment()

            let stdout = Pipe()
            let stderr = Pipe()
            process.standardOutput = stdout
            process.standardError = stderr

            try process.run()
            let out = stdout.fileHandleForReading.readDataToEndOfFile()
            let err = stderr.fileHandleForReading.readDataToEndOfFile()
            process.waitUntilExit()

            if process.terminationStatus != 0 {
                let stderrText = String(data: err, encoding: .utf8) ?? ""
                throw AutotaskError.commandFailed(
                    command: "autotask \(arguments.joined(separator: " "))",
                    code: process.terminationStatus,
                    stderr: stderrText
                )
            }
            return out
        }.value
    }

    private func locateExecutable() throws -> URL {
        let env = ProcessInfo.processInfo.environment
        let candidates = [
            env["AUTOTASK_CLI"],
            "/opt/homebrew/bin/autotask",
            "/usr/local/bin/autotask",
            "\(env["HOME"] ?? "")/.local/bin/autotask"
        ].compactMap { $0 }.filter { !$0.isEmpty }

        for path in candidates where FileManager.default.isExecutableFile(atPath: path) {
            return URL(fileURLWithPath: path)
        }
        throw AutotaskError.cliNotFound
    }

    private static func environment() -> [String: String] {
        var env = ProcessInfo.processInfo.environment
        let home = env["HOME"] ?? NSHomeDirectory()
        env["HOME"] = home
        env["LANG"] = env["LANG"] ?? "en_US.UTF-8"
        env["PATH"] = [
            "/opt/homebrew/bin",
            "/usr/local/bin",
            "\(home)/.local/bin",
            "/usr/bin",
            "/bin",
            "/usr/sbin",
            "/sbin"
        ].joined(separator: ":")
        return env
    }
}
