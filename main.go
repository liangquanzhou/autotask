package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

const appVersion = "0.1.1"

type Config struct {
	Tasks []Task `yaml:"tasks" json:"tasks"`
}

type Task struct {
	Name             string            `yaml:"name" json:"name"`
	Label            string            `yaml:"label" json:"label"`
	Title            string            `yaml:"title" json:"title"`
	Kind             string            `yaml:"kind" json:"kind"`
	Managed          bool              `yaml:"managed" json:"managed"`
	Enabled          *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	RunAtLoad        bool              `yaml:"run_at_load,omitempty" json:"run_at_load,omitempty"`
	KeepAlive        bool              `yaml:"keep_alive,omitempty" json:"keep_alive,omitempty"`
	LowPriorityIO    bool              `yaml:"low_priority_io,omitempty" json:"low_priority_io,omitempty"`
	Nice             *int              `yaml:"nice,omitempty" json:"nice,omitempty"`
	ThrottleInterval *int              `yaml:"throttle_interval,omitempty" json:"throttle_interval,omitempty"`
	ProcessType      string            `yaml:"process_type,omitempty" json:"process_type,omitempty"`
	Schedule         Schedule          `yaml:"schedule" json:"schedule"`
	Command          []string          `yaml:"command" json:"command"`
	WorkingDirectory string            `yaml:"working_directory" json:"working_directory,omitempty"`
	Env              map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Log              string            `yaml:"log" json:"log,omitempty"`
	Stdout           string            `yaml:"stdout,omitempty" json:"stdout,omitempty"`
	Stderr           string            `yaml:"stderr,omitempty" json:"stderr,omitempty"`
	Tags             []string          `yaml:"tags" json:"tags,omitempty"`
	Notes            string            `yaml:"notes" json:"notes,omitempty"`
}

type Schedule struct {
	Type         string `yaml:"type" json:"type"`
	Cron         string `yaml:"cron,omitempty" json:"cron,omitempty"`
	EverySeconds int    `yaml:"every_seconds,omitempty" json:"every_seconds,omitempty"`
	Minute       *int   `yaml:"minute,omitempty" json:"minute,omitempty"`
	Hour         *int   `yaml:"hour,omitempty" json:"hour,omitempty"`
	Day          *int   `yaml:"day,omitempty" json:"day,omitempty"`
	Weekday      *int   `yaml:"weekday,omitempty" json:"weekday,omitempty"`
	Month        *int   `yaml:"month,omitempty" json:"month,omitempty"`
}

type DiscoveredTask struct {
	Source   string `json:"source"`
	Label    string `json:"label"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Managed  bool   `json:"managed"`
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
}

type DoctorIssue struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Ref     string `json:"ref,omitempty"`
}

type ScanResult struct {
	Items  []DiscoveredTask `json:"items"`
	Issues []DoctorIssue    `json:"issues,omitempty"`
}

type DiffAction struct {
	Action  string `json:"action"`
	Task    string `json:"task"`
	Label   string `json:"label"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason"`
	Current string `json:"current,omitempty"`
	Desired string `json:"desired,omitempty"`
}

type DiffResult struct {
	Actions []DiffAction  `json:"actions"`
	Issues  []DoctorIssue `json:"issues,omitempty"`
}

type StatusRow struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Enabled  bool   `json:"enabled"`
	Status   string `json:"status"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Path     string `json:"path,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := os.Args[1]
	jsonOut := hasFlag(os.Args[2:], "--json")
	personalOnly := hasFlag(os.Args[2:], "--personal")

	var err error
	switch cmd {
	case "scan":
		err = runScan(jsonOut, personalOnly)
	case "doctor":
		err = runDoctor(jsonOut, personalOnly)
	case "list":
		err = runList(jsonOut)
	case "status":
		err = runStatus(os.Args[2:], jsonOut)
	case "ui-state":
		err = runUIState(jsonOut)
	case "diff":
		err = runDiff(jsonOut)
	case "sync":
		err = runSync(os.Args[2:], jsonOut)
	case "import":
		err = runImport(os.Args[2:], jsonOut)
	case "run":
		err = runRun(os.Args[2:])
	case "logs":
		err = runLogs(os.Args[2:])
	case "edit":
		err = runEdit(os.Args[2:])
	case "fix-cron-dupes":
		err = runFixCronDupes(os.Args[2:], jsonOut)
	case "enable":
		err = runSetLoaded(os.Args[2:], true)
	case "disable":
		err = runSetLoaded(os.Args[2:], false)
	case "init":
		err = runInit()
	case "help", "-h", "--help":
		usage()
	case "version", "--version":
		fmt.Println(appVersion)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "autotask:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`autotask manages a personal registry of macOS automation tasks.

Usage:
  autotask scan [--json] [--personal]
                            scan crontab, launchd, and brew services
  autotask doctor [--json] [--personal]
                            report duplicates, invalid plists, and config drift
  autotask list [--json]     list tasks from ~/.config/autotask/tasks.yaml
  autotask status [name] [--json]
                            show registered task status
  autotask ui-state [--json] aggregate status, diff, and doctor for a UI
  autotask diff [--json]     compare tasks.yaml with user LaunchAgents
  autotask sync [--apply] [--json]
                            generate/load user LaunchAgents; dry-run by default
  autotask import [--apply] [--json]
                            import personal scan results into tasks.yaml; preview by default
  autotask run <name>        run a registered task once
  autotask logs <name> [-n N]
                            print recent task log lines
  autotask edit [name]       open tasks.yaml in $EDITOR
  autotask fix-cron-dupes [--apply] [--json]
                            remove crontab entries duplicated by registered launchd tasks
  autotask enable <name>     launchctl bootstrap a registered launchd task
  autotask disable <name>    launchctl bootout a registered launchd task
  autotask init              create ~/.config/autotask with a starter tasks.yaml`)
}

func runScan(jsonOut, personalOnly bool) error {
	result := scanAll()
	sortTasks(result.Items)
	if personalOnly {
		result.Items = filterPersonal(result.Items)
	}
	if jsonOut {
		return writeJSON(result)
	}
	printTasks(result.Items)
	if len(result.Issues) > 0 {
		fmt.Println()
		printIssues(result.Issues)
	}
	return nil
}

func runDoctor(jsonOut, personalOnly bool) error {
	result := scanAll()
	cfg, cfgPath, err := loadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		result.Issues = append(result.Issues, DoctorIssue{
			Level: "error", Code: "config_invalid", Message: err.Error(), Ref: cfgPath,
		})
	}
	result.Issues = append(result.Issues, doctor(result.Items, cfg, cfgPath)...)
	sortTasks(result.Items)
	sortIssues(result.Issues)
	if personalOnly {
		result.Items = filterPersonal(result.Items)
	}

	if jsonOut {
		return writeJSON(result)
	}
	printTasks(result.Items)
	fmt.Println()
	if len(result.Issues) == 0 {
		fmt.Println("No doctor issues found.")
		return nil
	}
	printIssues(result.Issues)
	return nil
}

func runList(jsonOut bool) error {
	cfg, cfgPath, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config not found: %s; run `autotask init` first", cfgPath)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(cfg)
	}
	if len(cfg.Tasks) == 0 {
		fmt.Println("No tasks registered.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLABEL\tSCHEDULE\tCOMMAND")
	for _, t := range cfg.Tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Name, t.Label, formatConfigSchedule(t.Schedule), strings.Join(t.Command, " "))
	}
	return w.Flush()
}

func runStatus(args []string, jsonOut bool) error {
	rows, err := statusRows(firstArg(args))
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(rows)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tENABLED\tLABEL\tSCHEDULE\tCOMMAND")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%t\t%s\t%s\t%s\n", row.Name, emptyDash(row.Status), row.Enabled, row.Label, emptyDash(row.Schedule), truncate(row.Command, 90))
	}
	return w.Flush()
}

func statusRows(name string) ([]StatusRow, error) {
	cfg, cfgPath, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config not found: %s; run `autotask init` first", cfgPath)
	}
	if err != nil {
		return nil, err
	}
	scan := scanAll()
	byLabel := map[string]DiscoveredTask{}
	for _, item := range scan.Items {
		byLabel[item.Label] = item
	}
	var rows []StatusRow
	for _, task := range cfg.Tasks {
		if name != "" && task.Name != name && task.Label != name {
			continue
		}
		row := StatusRow{
			Name: task.Name, Label: task.Label, Kind: task.Kind, Enabled: taskEnabled(task),
			Schedule: formatConfigSchedule(task.Schedule), Command: strings.Join(task.Command, " "),
			Path: launchAgentPath(task.Label),
		}
		if item, ok := byLabel[task.Label]; ok {
			row.Status = item.Status
			if !item.Valid {
				row.Status = "invalid"
			}
			if row.Path == "" {
				row.Path = item.Path
			}
		} else {
			row.Status = "not-installed"
		}
		rows = append(rows, row)
	}
	if name != "" && len(rows) == 0 {
		return nil, fmt.Errorf("task not found: %s", name)
	}
	return rows, nil
}

func runUIState(jsonOut bool) error {
	status, err := statusRows("")
	if err != nil {
		return err
	}
	diff, err := diffConfig()
	if err != nil {
		return err
	}
	state := map[string]any{
		"version": appVersion,
		"config":  configPath(),
		"status":  status,
		"diff":    diff.Actions,
		"issues":  diff.Issues,
	}
	if jsonOut {
		return writeJSON(state)
	}
	return writeJSON(state)
}

func runDiff(jsonOut bool) error {
	result, err := diffConfig()
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(result)
	}
	printDiff(result.Actions)
	if len(result.Issues) > 0 {
		fmt.Println()
		printIssues(result.Issues)
	}
	return nil
}

func runSync(args []string, jsonOut bool) error {
	apply := hasFlag(args, "--apply")
	result, err := diffConfig()
	if err != nil {
		return err
	}
	if !apply {
		if jsonOut {
			return writeJSON(result)
		}
		printDiff(result.Actions)
		fmt.Println("\nDry run only. Re-run with `sync --apply` to write plists and update launchd.")
		return nil
	}
	var applied []DiffAction
	appliedLabels := map[string]bool{}
	for _, action := range result.Actions {
		if action.Action == "noop" {
			continue
		}
		if action.Action == "bootstrap" && appliedLabels[action.Label] {
			continue
		}
		task, err := taskByNameOrLabel(action.Task)
		if err != nil {
			return err
		}
		if task.Kind != "" && task.Kind != "launchd" {
			continue
		}
		if action.Action == "remove" {
			if err := bootoutTask(task); err != nil && !isLaunchctlNotLoaded(err) {
				return err
			}
			if err := os.Remove(launchAgentPath(task.Label)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			applied = append(applied, action)
			appliedLabels[action.Label] = true
			continue
		}
		if err := writeTaskPlist(task); err != nil {
			return err
		}
		if taskEnabled(task) {
			_ = bootoutTask(task)
			if err := bootstrapTask(task); err != nil {
				return err
			}
		} else {
			_ = bootoutTask(task)
		}
		applied = append(applied, action)
		appliedLabels[action.Label] = true
	}
	out := map[string]any{"applied": applied, "planned": result.Actions}
	if jsonOut {
		return writeJSON(out)
	}
	if len(applied) == 0 {
		fmt.Println("No changes applied.")
		return nil
	}
	fmt.Println("Applied:")
	printDiff(applied)
	return nil
}

func runImport(args []string, jsonOut bool) error {
	apply := hasFlag(args, "--apply")
	refresh := hasFlag(args, "--refresh")
	cfg, cfgPath, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		cfg = Config{}
		cfgPath = configPath()
	} else if err != nil {
		return err
	}
	existing := map[string]int{}
	for i, task := range cfg.Tasks {
		existing[task.Label] = i
	}
	scan := scanAll()
	var imported []Task
	for _, item := range scan.Items {
		if !item.Managed || item.Source != "launchd" || !item.Valid || item.Label == "" {
			continue
		}
		idx, exists := existing[item.Label]
		if exists && !refresh {
			continue
		}
		task := taskFromDiscovered(item)
		if task.Name == "" {
			continue
		}
		imported = append(imported, task)
		if apply {
			if exists {
				cfg.Tasks[idx] = task
			} else {
				cfg.Tasks = append(cfg.Tasks, task)
				existing[task.Label] = len(cfg.Tasks) - 1
			}
		}
	}
	sort.Slice(imported, func(i, j int) bool { return imported[i].Name < imported[j].Name })
	if jsonOut {
		return writeJSON(map[string]any{"apply": apply, "refresh": refresh, "config": cfgPath, "imported": imported})
	}
	if len(imported) == 0 {
		fmt.Println("No new personal launchd tasks to import.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLABEL\tSCHEDULE\tCOMMAND")
	for _, task := range imported {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", task.Name, task.Label, formatConfigSchedule(task.Schedule), strings.Join(task.Command, " "))
	}
	_ = w.Flush()
	if !apply {
		fmt.Println("\nPreview only. Re-run with `import --apply` to update tasks.yaml.")
		return nil
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Println("\nUpdated:", cfgPath)
	return nil
}

func runRun(args []string) error {
	name := firstArg(args)
	if name == "" {
		return errors.New("usage: autotask run <name>")
	}
	task, err := taskByNameOrLabel(name)
	if err != nil {
		return err
	}
	if len(task.Command) == 0 {
		return fmt.Errorf("task has no command: %s", task.Name)
	}
	cmd := exec.Command(expandHome(task.Command[0]), task.Command[1:]...)
	if task.WorkingDirectory != "" {
		cmd.Dir = expandHome(task.WorkingDirectory)
	}
	cmd.Env = taskEnv(task)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runLogs(args []string) error {
	name := firstArg(args)
	if name == "" {
		return errors.New("usage: autotask logs <name> [-n N]")
	}
	n := 100
	if raw := flagValue(args, "-n"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return fmt.Errorf("invalid -n value: %s", raw)
		}
		n = parsed
	}
	task, err := taskByNameOrLabel(name)
	if err != nil {
		return err
	}
	logPath := taskLogPath(task, false)
	if logPath == "" {
		return fmt.Errorf("task has no log path: %s", task.Name)
	}
	if _, err := os.Stat(expandHome(logPath)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("No log file yet: %s\n", expandHome(logPath))
			return nil
		}
		return err
	}
	cmd := exec.Command("tail", "-n", strconv.Itoa(n), expandHome(logPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runEdit(args []string) error {
	path := configPath()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runFixCronDupes(args []string, jsonOut bool) error {
	apply := hasFlag(args, "--apply")
	result, err := planCronDupeRemoval()
	if err != nil {
		return err
	}
	if jsonOut {
		out := map[string]any{"apply": apply, "remove": result.Remove, "keep": result.Keep}
		if !apply {
			out["message"] = "dry run only; re-run with --apply"
		}
		return writeJSON(out)
	}
	if len(result.Remove) == 0 {
		fmt.Println("No duplicate crontab entries found.")
		return nil
	}
	fmt.Println("Duplicate crontab entries:")
	for _, line := range result.Remove {
		fmt.Println("-", line)
	}
	if !apply {
		fmt.Println("\nDry run only. Re-run with `fix-cron-dupes --apply` to update crontab.")
		return nil
	}
	if err := installCrontabLines(result.Keep); err != nil {
		return err
	}
	fmt.Printf("\nUpdated crontab; removed %d duplicate entr", len(result.Remove))
	if len(result.Remove) == 1 {
		fmt.Println("y.")
	} else {
		fmt.Println("ies.")
	}
	return nil
}

type CronDupePlan struct {
	Remove []string `json:"remove"`
	Keep   []string `json:"keep"`
}

func planCronDupeRemoval() (CronDupePlan, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return CronDupePlan{}, err
	}
	registered := map[string]bool{}
	for _, task := range cfg.Tasks {
		if task.Kind != "" && task.Kind != "launchd" {
			continue
		}
		key := normalizedCommandKey(strings.Join(task.Command, " "))
		if key != "" {
			registered[key] = true
		}
	}
	out, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" || strings.Contains(strings.ToLower(text), "no crontab") {
			return CronDupePlan{}, nil
		}
		return CronDupePlan{}, errors.New(text)
	}
	var plan CronDupePlan
	var pendingComments []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			pendingComments = append(pendingComments, line)
			continue
		}
		cmd := crontabCommand(line)
		key := normalizedCommandKey(cmd)
		if key != "" && registered[key] {
			plan.Remove = append(plan.Remove, pendingComments...)
			plan.Remove = append(plan.Remove, line)
			pendingComments = nil
			continue
		}
		plan.Keep = append(plan.Keep, pendingComments...)
		pendingComments = nil
		plan.Keep = append(plan.Keep, line)
	}
	plan.Keep = append(plan.Keep, pendingComments...)
	return plan, nil
}

func crontabCommand(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}
	if strings.HasPrefix(fields[0], "@") {
		return strings.Join(fields[1:], " ")
	}
	if len(fields) < 6 {
		return ""
	}
	return strings.Join(fields[5:], " ")
}

func installCrontabLines(lines []string) error {
	var content string
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab -: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runSetLoaded(args []string, enabled bool) error {
	name := firstArg(args)
	if name == "" {
		if enabled {
			return errors.New("usage: autotask enable <name>")
		}
		return errors.New("usage: autotask disable <name>")
	}
	task, err := taskByNameOrLabel(name)
	if err != nil {
		return err
	}
	if task.Kind != "" && task.Kind != "launchd" {
		return fmt.Errorf("task is not launchd-backed: %s", task.Name)
	}
	if enabled {
		if err := writeTaskPlist(task); err != nil {
			return err
		}
		return bootstrapTask(task)
	}
	err = bootoutTask(task)
	if isLaunchctlNotLoaded(err) {
		return nil
	}
	return err
}

func runInit() error {
	dir := configDir()
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "generated"), 0o755); err != nil {
		return err
	}
	path := configPath()
	if _, err := os.Stat(path); err == nil {
		fmt.Println("Config already exists:", path)
		return nil
	}
	starter := Config{Tasks: []Task{}}
	data, err := yaml.Marshal(starter)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Println("Created:", path)
	return nil
}

func scanAll() ScanResult {
	var result ScanResult
	add := func(items []DiscoveredTask, issues []DoctorIssue) {
		result.Items = append(result.Items, items...)
		result.Issues = append(result.Issues, issues...)
	}
	add(scanCrontab())
	add(scanLaunchd())
	add(scanBrewServices())
	return result
}

func scanCrontab() ([]DiscoveredTask, []DoctorIssue) {
	out, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(text), "no crontab") || text == "" {
			return nil, nil
		}
		return nil, []DoctorIssue{{Level: "warn", Code: "crontab_read_failed", Message: text}}
	}
	var items []DiscoveredTask
	for i, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(line, "=") && !strings.Contains(line, " ") {
			continue
		}
		item := DiscoveredTask{Source: "crontab", Label: fmt.Sprintf("crontab:%d", i+1), Name: fmt.Sprintf("crontab line %d", i+1), Valid: true}
		if strings.HasPrefix(line, "@") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				item.Valid = false
				item.Error = "invalid crontab shortcut"
			} else {
				item.Schedule = parts[0]
				item.Command = strings.Join(parts[1:], " ")
			}
		} else {
			parts := strings.Fields(line)
			if len(parts) < 6 {
				item.Valid = false
				item.Error = "invalid crontab line"
				item.Command = line
			} else {
				item.Schedule = strings.Join(parts[:5], " ")
				item.Command = strings.Join(parts[5:], " ")
			}
		}
		item.Managed = looksPersonal(item.Label, item.Command, "")
		items = append(items, item)
	}
	return items, nil
}

func scanLaunchd() ([]DiscoveredTask, []DoctorIssue) {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, "Library", "LaunchAgents"),
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
	}
	loaded := loadedLaunchdLabels()
	var items []DiscoveredTask
	var issues []DoctorIssue
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.plist"))
		for _, path := range matches {
			item, err := parseLaunchdPlist(path)
			if err != nil {
				label := strings.TrimSuffix(filepath.Base(path), ".plist")
				item = DiscoveredTask{
					Source: "launchd", Label: label, Name: label, Path: path, Valid: false, Error: err.Error(), Managed: looksPersonal(label, "", path),
				}
				if recovered, ok := recoverLoadedLaunchd(label, path); ok {
					item.Schedule = recovered.Schedule
					item.Command = recovered.Command
					item.Status = recovered.Status
				}
				items = append(items, item)
				issues = append(issues, DoctorIssue{
					Level: "error", Code: "invalid_plist", Message: err.Error(), Ref: path,
				})
				continue
			}
			item.Status = loaded[item.Label]
			if item.Status == "" {
				item.Status = "not-loaded"
			}
			item.Managed = looksPersonal(item.Label, item.Command, path)
			items = append(items, item)
		}
	}
	return items, issues
}

func recoverLoadedLaunchd(label, path string) (DiscoveredTask, bool) {
	uid := os.Getuid()
	for _, domain := range []string{fmt.Sprintf("gui/%d", uid), "system"} {
		out, err := exec.Command("launchctl", "print", domain+"/"+label).CombinedOutput()
		if err != nil {
			continue
		}
		text := string(out)
		item := DiscoveredTask{
			Source: "launchd", Label: label, Name: label, Path: path, Valid: false, Status: "loaded",
		}
		if strings.Contains(text, "state = not running") {
			item.Status = "loaded"
		}
		if pid := firstRegexpSubmatch(text, `(?m)^\s*pid = ([0-9]+)$`); pid != "" {
			item.Status = "running pid=" + pid
		}
		item.Command = parseLaunchctlArguments(text)
		item.Schedule = parseLaunchctlCalendar(text)
		return item, true
	}
	return DiscoveredTask{}, false
}

func parseLaunchctlArguments(text string) string {
	start := strings.Index(text, "arguments = {")
	if start == -1 {
		if p := firstRegexpSubmatch(text, `(?m)^\s*program = (.+)$`); p != "" {
			return strings.TrimSpace(p)
		}
		return ""
	}
	body := text[start:]
	end := strings.Index(body, "\n\t}")
	if end != -1 {
		body = body[:end]
	}
	var args []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "arguments = {" {
			continue
		}
		args = append(args, line)
	}
	return strings.Join(args, " ")
}

func parseLaunchctlCalendar(text string) string {
	if !strings.Contains(text, "com.apple.launchd.calendarinterval") {
		return ""
	}
	var parts []string
	for _, key := range []string{"Month", "Day", "Weekday", "Hour", "Minute"} {
		if val := firstRegexpSubmatch(text, `"`+key+`" => ([0-9]+)`); val != "" {
			parts = append(parts, strings.ToLower(key)+"="+val)
		}
	}
	if len(parts) == 0 {
		return "calendar"
	}
	return "calendar " + strings.Join(parts, " ")
}

func parseLaunchdPlist(path string) (DiscoveredTask, error) {
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return DiscoveredTask{}, errors.New(msg)
	}
	var pl map[string]any
	if err := json.Unmarshal(out, &pl); err != nil {
		return DiscoveredTask{}, err
	}
	label := str(pl["Label"])
	if label == "" {
		label = strings.TrimSuffix(filepath.Base(path), ".plist")
	}
	cmd := launchdCommand(pl)
	return DiscoveredTask{
		Source:   "launchd",
		Label:    label,
		Name:     label,
		Schedule: launchdSchedule(pl),
		Command:  cmd,
		Path:     path,
		Valid:    true,
	}, nil
}

func scanBrewServices() ([]DiscoveredTask, []DoctorIssue) {
	_, err := exec.LookPath("brew")
	if err != nil {
		return nil, nil
	}
	out, err := exec.Command("brew", "services", "list").CombinedOutput()
	if err != nil {
		return nil, []DoctorIssue{{Level: "warn", Code: "brew_services_failed", Message: strings.TrimSpace(string(out))}}
	}
	var items []DiscoveredTask
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name, status := fields[0], fields[1]
		file := ""
		for _, f := range fields {
			if strings.HasSuffix(f, ".plist") || strings.Contains(f, "/LaunchAgents/") || strings.Contains(f, "/LaunchDaemons/") {
				file = f
			}
		}
		items = append(items, DiscoveredTask{
			Source: "brew", Label: "homebrew.mxcl." + name, Name: name, Status: status, Path: file, Valid: true, Managed: false,
		})
	}
	return items, nil
}

func loadedLaunchdLabels() map[string]string {
	status := map[string]string{}
	uid := os.Getuid()
	for _, domain := range []string{fmt.Sprintf("gui/%d", uid), "system"} {
		out, err := exec.Command("launchctl", "print", domain).Output()
		if err != nil {
			continue
		}
		inServices := false
		for _, line := range strings.Split(string(out), "\n") {
			s := strings.TrimSpace(line)
			if s == "services = {" {
				inServices = true
				continue
			}
			if inServices && s == "}" {
				inServices = false
				continue
			}
			if !inServices || s == "" {
				continue
			}
			fields := strings.Fields(s)
			if len(fields) < 2 {
				continue
			}
			label := fields[len(fields)-1]
			if strings.Contains(label, ".") {
				if fields[0] != "0" && isNumber(fields[0]) {
					status[label] = "running pid=" + fields[0]
				} else if status[label] == "" {
					status[label] = "loaded"
				}
			}
		}
	}
	return status
}

func doctor(items []DiscoveredTask, cfg Config, cfgPath string) []DoctorIssue {
	var issues []DoctorIssue
	if cfgPath != "" {
		if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
			issues = append(issues, DoctorIssue{
				Level: "info", Code: "config_missing", Message: "no registry exists yet; run `autotask init`", Ref: cfgPath,
			})
		}
	}

	bySig := map[string][]DiscoveredTask{}
	for _, item := range items {
		if item.Command == "" {
			continue
		}
		sig := commandSignature(item.Command)
		bySig[sig] = append(bySig[sig], item)
	}
	for _, group := range bySig {
		if len(group) < 2 {
			continue
		}
		refs := make([]string, 0, len(group))
		for _, item := range group {
			refs = append(refs, fmt.Sprintf("%s:%s", item.Source, item.Label))
		}
		issues = append(issues, DoctorIssue{
			Level: "warn", Code: "duplicate_command", Message: "same command appears in multiple schedulers: " + strings.Join(refs, ", "),
		})
	}

	seenLabels := map[string]DiscoveredTask{}
	for _, item := range items {
		if item.Label != "" {
			seenLabels[item.Label] = item
		}
	}
	for _, task := range cfg.Tasks {
		if task.Name == "" {
			issues = append(issues, DoctorIssue{Level: "warn", Code: "task_missing_name", Message: "registered task is missing name", Ref: cfgPath})
		}
		if task.Label == "" {
			issues = append(issues, DoctorIssue{Level: "warn", Code: "task_missing_label", Message: "registered task " + task.Name + " is missing label", Ref: cfgPath})
		} else if _, ok := seenLabels[task.Label]; !ok {
			issues = append(issues, DoctorIssue{Level: "warn", Code: "task_not_loaded", Message: "registered task label not found in scan: " + task.Label, Ref: cfgPath})
		}
		if len(task.Command) == 0 {
			issues = append(issues, DoctorIssue{Level: "warn", Code: "task_missing_command", Message: "registered task has no command: " + task.Name, Ref: cfgPath})
		} else {
			checkExecutable(task, cfgPath, &issues)
		}
		if task.Log != "" {
			dir := filepath.Dir(expandHome(task.Log))
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				issues = append(issues, DoctorIssue{Level: "warn", Code: "log_dir_missing", Message: "log directory does not exist for task: " + task.Name, Ref: dir})
			}
		}
	}
	return issues
}

func diffConfig() (DiffResult, error) {
	cfg, cfgPath, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return DiffResult{}, fmt.Errorf("config not found: %s; run `autotask init` first", cfgPath)
	}
	if err != nil {
		return DiffResult{}, err
	}
	scan := scanAll()
	issues := doctor(scan.Items, cfg, cfgPath)
	byLabel := map[string]DiscoveredTask{}
	for _, item := range scan.Items {
		byLabel[item.Label] = item
	}
	var actions []DiffAction
	for _, task := range cfg.Tasks {
		if task.Kind != "" && task.Kind != "launchd" {
			actions = append(actions, DiffAction{Action: "skip", Task: task.Name, Label: task.Label, Reason: "non-launchd tasks are read-only in this version"})
			continue
		}
		if task.Label == "" {
			actions = append(actions, DiffAction{Action: "error", Task: task.Name, Reason: "missing label"})
			continue
		}
		desired, err := renderLaunchdPlist(task)
		if err != nil {
			actions = append(actions, DiffAction{Action: "error", Task: task.Name, Label: task.Label, Reason: err.Error()})
			continue
		}
		path := launchAgentPath(task.Label)
		current, readErr := os.ReadFile(path)
		item, installed := byLabel[task.Label]
		if errors.Is(readErr, os.ErrNotExist) {
			actions = append(actions, DiffAction{Action: "create", Task: task.Name, Label: task.Label, Path: path, Reason: "plist missing", Desired: desired})
		} else if readErr != nil {
			actions = append(actions, DiffAction{Action: "error", Task: task.Name, Label: task.Label, Path: path, Reason: readErr.Error()})
		} else if installed && item.Valid && taskSemanticallyEqual(task, item) {
			// Existing plist may have different XML formatting or key ordering. Treat it
			// as in-sync when the fields autotask manages are equivalent.
		} else if normalizeXML(string(current)) != normalizeXML(desired) {
			reason := "plist differs from registry"
			if installed && !item.Valid {
				reason = "plist is invalid and will be regenerated"
			}
			actions = append(actions, DiffAction{Action: "update", Task: task.Name, Label: task.Label, Path: path, Reason: reason, Current: string(current), Desired: desired})
		}
		if installed {
			loaded := item.Status != "" && item.Status != "not-loaded"
			if taskEnabled(task) && !loaded {
				actions = append(actions, DiffAction{Action: "bootstrap", Task: task.Name, Label: task.Label, Path: path, Reason: "enabled task is not loaded"})
			}
			if !taskEnabled(task) && loaded {
				actions = append(actions, DiffAction{Action: "bootout", Task: task.Name, Label: task.Label, Path: path, Reason: "disabled task is loaded"})
			}
		} else if taskEnabled(task) {
			actions = append(actions, DiffAction{Action: "bootstrap", Task: task.Name, Label: task.Label, Path: path, Reason: "enabled task is not installed"})
		}
	}
	if len(actions) == 0 {
		actions = append(actions, DiffAction{Action: "noop", Reason: "registry and launchd files are in sync"})
	}
	return DiffResult{Actions: actions, Issues: issues}, nil
}

func taskSemanticallyEqual(desired Task, currentItem DiscoveredTask) bool {
	current, err := taskFromLaunchdPlist(currentItem)
	if err != nil {
		return false
	}
	desiredComparable := comparableTask(desired)
	currentComparable := comparableTask(current)
	a, _ := json.Marshal(desiredComparable)
	b, _ := json.Marshal(currentComparable)
	return bytes.Equal(a, b)
}

func comparableTask(t Task) map[string]any {
	return map[string]any{
		"label":             t.Label,
		"schedule":          t.Schedule,
		"command":           t.Command,
		"working_directory": expandHome(t.WorkingDirectory),
		"env":               t.Env,
		"run_at_load":       t.RunAtLoad,
		"keep_alive":        t.KeepAlive,
		"low_priority_io":   t.LowPriorityIO,
		"nice":              t.Nice,
		"throttle_interval": t.ThrottleInterval,
		"process_type":      t.ProcessType,
		"stdout":            taskLogPath(t, false),
		"stderr":            taskLogPath(t, true),
	}
}

func printDiff(actions []DiffAction) {
	if len(actions) == 0 {
		fmt.Println("No changes.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ACTION\tTASK\tLABEL\tREASON\tPATH")
	for _, action := range actions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", action.Action, action.Task, action.Label, action.Reason, action.Path)
	}
	_ = w.Flush()
}

func taskByNameOrLabel(name string) (Task, error) {
	cfg, cfgPath, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return Task{}, fmt.Errorf("config not found: %s; run `autotask init` first", cfgPath)
	}
	if err != nil {
		return Task{}, err
	}
	for _, task := range cfg.Tasks {
		if task.Name == name || task.Label == name {
			return task, nil
		}
	}
	return Task{}, fmt.Errorf("task not found: %s", name)
}

func saveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sort.Slice(cfg.Tasks, func(i, j int) bool { return cfg.Tasks[i].Name < cfg.Tasks[j].Name })
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func taskFromDiscovered(item DiscoveredTask) Task {
	if item.Source == "launchd" && item.Valid && item.Path != "" {
		if task, err := taskFromLaunchdPlist(item); err == nil {
			return task
		}
	}
	name := item.Label
	for _, prefix := range personalPrefixes() {
		name = strings.TrimPrefix(name, prefix)
	}
	return Task{
		Name: name, Label: item.Label, Title: name, Kind: "launchd", Managed: true, Enabled: boolPtr(true),
		Schedule: parseDisplaySchedule(item.Schedule),
		Command:  commandFields(item.Command),
		Log:      filepath.Join(configDir(), "logs", name+".log"),
		Notes:    "Imported from existing launchd scan.",
	}
}

func taskFromLaunchdPlist(item DiscoveredTask) (Task, error) {
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", item.Path).CombinedOutput()
	if err != nil {
		return Task{}, errors.New(strings.TrimSpace(string(out)))
	}
	var pl map[string]any
	if err := json.Unmarshal(out, &pl); err != nil {
		return Task{}, err
	}
	name := item.Label
	for _, prefix := range personalPrefixes() {
		name = strings.TrimPrefix(name, prefix)
	}
	task := Task{
		Name: name, Label: item.Label, Title: name, Kind: "launchd", Managed: true, Enabled: boolPtr(true),
		Schedule:         scheduleFromLaunchdPlist(pl),
		Command:          programArgsFromPlist(pl),
		WorkingDirectory: str(pl["WorkingDirectory"]),
		RunAtLoad:        boolValue(pl["RunAtLoad"]),
		KeepAlive:        pl["KeepAlive"] != nil && fmt.Sprint(pl["KeepAlive"]) != "false",
		LowPriorityIO:    boolValue(pl["LowPriorityIO"]),
		ProcessType:      str(pl["ProcessType"]),
		Stdout:           str(pl["StandardOutPath"]),
		Stderr:           str(pl["StandardErrorPath"]),
		Notes:            "Imported from existing launchd scan.",
	}
	if task.Stdout != "" && task.Stdout == task.Stderr {
		task.Log = task.Stdout
		task.Stdout = ""
		task.Stderr = ""
	}
	if n, ok := intValue(pl["Nice"]); ok {
		task.Nice = intPtr(n)
	}
	if n, ok := intValue(pl["ThrottleInterval"]); ok {
		task.ThrottleInterval = intPtr(n)
	}
	if env, ok := pl["EnvironmentVariables"].(map[string]any); ok {
		task.Env = map[string]string{}
		for k, v := range env {
			task.Env[k] = str(v)
		}
	}
	return task, nil
}

func programArgsFromPlist(pl map[string]any) []string {
	if args, ok := pl["ProgramArguments"].([]any); ok {
		out := make([]string, 0, len(args))
		for _, arg := range args {
			out = append(out, str(arg))
		}
		return out
	}
	if p := str(pl["Program"]); p != "" {
		return []string{p}
	}
	return nil
}

func scheduleFromLaunchdPlist(pl map[string]any) Schedule {
	if si, ok := intValue(pl["StartInterval"]); ok {
		return Schedule{Type: "interval", EverySeconds: si}
	}
	if sci, ok := pl["StartCalendarInterval"]; ok {
		if arr, ok := sci.([]any); ok && len(arr) > 0 {
			sci = arr[0]
		}
		if m, ok := sci.(map[string]any); ok {
			s := Schedule{Type: "calendar"}
			if v, ok := intValue(m["Month"]); ok {
				s.Month = intPtr(v)
			}
			if v, ok := intValue(m["Day"]); ok {
				s.Day = intPtr(v)
			}
			if v, ok := intValue(m["Weekday"]); ok {
				s.Weekday = intPtr(v)
			}
			if v, ok := intValue(m["Hour"]); ok {
				s.Hour = intPtr(v)
			}
			if v, ok := intValue(m["Minute"]); ok {
				s.Minute = intPtr(v)
			}
			return s
		}
	}
	if boolValue(pl["RunAtLoad"]) || pl["KeepAlive"] != nil {
		return Schedule{Type: "daemon"}
	}
	return Schedule{}
}

func parseDisplaySchedule(schedule string) Schedule {
	schedule = strings.TrimSpace(schedule)
	if strings.HasPrefix(schedule, "every ") && strings.HasSuffix(schedule, "s") {
		raw := strings.TrimSuffix(strings.TrimPrefix(schedule, "every "), "s")
		n, _ := strconv.Atoi(raw)
		return Schedule{Type: "interval", EverySeconds: n}
	}
	if strings.HasPrefix(schedule, "calendar ") {
		s := Schedule{Type: "calendar"}
		for _, part := range strings.Fields(strings.TrimPrefix(schedule, "calendar ")) {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				continue
			}
			switch k {
			case "month":
				s.Month = intPtr(n)
			case "day":
				s.Day = intPtr(n)
			case "weekday":
				s.Weekday = intPtr(n)
			case "hour":
				s.Hour = intPtr(n)
			case "minute":
				s.Minute = intPtr(n)
			}
		}
		return s
	}
	if strings.Contains(schedule, "run-at-load") {
		return Schedule{Type: "daemon"}
	}
	return Schedule{}
}

func commandFields(command string) []string {
	if command == "" {
		return nil
	}
	return strings.Fields(command)
}

func writeTaskPlist(task Task) error {
	plist, err := renderLaunchdPlist(task)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(configDir(), "generated"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(launchAgentPath(task.Label)), 0o755); err != nil {
		return err
	}
	if logPath := taskLogPath(task, false); logPath != "" {
		if err := os.MkdirAll(filepath.Dir(expandHome(logPath)), 0o755); err != nil {
			return err
		}
	}
	generated := filepath.Join(configDir(), "generated", task.Label+".plist")
	if err := atomicWrite(generated, []byte(plist), 0o644); err != nil {
		return err
	}
	return atomicWrite(launchAgentPath(task.Label), []byte(plist), 0o644)
}

func renderLaunchdPlist(task Task) (string, error) {
	if task.Label == "" {
		return "", errors.New("missing label")
	}
	if len(task.Command) == 0 {
		return "", errors.New("missing command")
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	writePlistString(&b, "Label", task.Label)
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range task.Command {
		b.WriteString("\t\t<string>")
		writeEscaped(&b, expandHome(arg))
		b.WriteString("</string>\n")
	}
	b.WriteString("\t</array>\n")
	if task.WorkingDirectory != "" {
		writePlistString(&b, "WorkingDirectory", expandHome(task.WorkingDirectory))
	}
	writeSchedulePlist(&b, task)
	if task.RunAtLoad || task.Schedule.Type == "daemon" {
		writePlistBool(&b, "RunAtLoad", true)
	}
	if task.KeepAlive {
		writePlistBool(&b, "KeepAlive", true)
	}
	if task.LowPriorityIO {
		writePlistBool(&b, "LowPriorityIO", true)
	}
	if task.Nice != nil {
		writePlistIntIndented(&b, "Nice", *task.Nice, 1)
	}
	if task.ThrottleInterval != nil {
		writePlistIntIndented(&b, "ThrottleInterval", *task.ThrottleInterval, 1)
	}
	if task.ProcessType != "" {
		writePlistString(&b, "ProcessType", task.ProcessType)
	}
	env := map[string]string{}
	for k, v := range task.Env {
		env[k] = expandHome(v)
	}
	if task.Env == nil {
		env["PATH"] = defaultLaunchdPath()
	} else if _, ok := env["PATH"]; !ok {
		env["PATH"] = defaultLaunchdPath()
	}
	b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writePlistStringIndented(&b, k, env[k], 2)
	}
	b.WriteString("\t</dict>\n")
	if out := taskLogPath(task, false); out != "" {
		writePlistString(&b, "StandardOutPath", expandHome(out))
	}
	if errPath := taskLogPath(task, true); errPath != "" {
		writePlistString(&b, "StandardErrorPath", expandHome(errPath))
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String(), nil
}

func writeSchedulePlist(b *strings.Builder, task Task) {
	switch task.Schedule.Type {
	case "calendar":
		b.WriteString("\t<key>StartCalendarInterval</key>\n\t<dict>\n")
		if task.Schedule.Month != nil {
			writePlistIntIndented(b, "Month", *task.Schedule.Month, 2)
		}
		if task.Schedule.Day != nil {
			writePlistIntIndented(b, "Day", *task.Schedule.Day, 2)
		}
		if task.Schedule.Weekday != nil {
			writePlistIntIndented(b, "Weekday", *task.Schedule.Weekday, 2)
		}
		if task.Schedule.Hour != nil {
			writePlistIntIndented(b, "Hour", *task.Schedule.Hour, 2)
		}
		if task.Schedule.Minute != nil {
			writePlistIntIndented(b, "Minute", *task.Schedule.Minute, 2)
		}
		b.WriteString("\t</dict>\n")
	case "interval":
		if task.Schedule.EverySeconds > 0 {
			writePlistIntIndented(b, "StartInterval", task.Schedule.EverySeconds, 1)
		}
	}
}

func writePlistString(b *strings.Builder, key, value string) {
	writePlistStringIndented(b, key, value, 1)
}

func writePlistStringIndented(b *strings.Builder, key, value string, indent int) {
	tabs := strings.Repeat("\t", indent)
	b.WriteString(tabs + "<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n" + tabs + "<string>")
	writeEscaped(b, value)
	b.WriteString("</string>\n")
}

func writePlistIntIndented(b *strings.Builder, key string, value int, indent int) {
	tabs := strings.Repeat("\t", indent)
	b.WriteString(tabs + "<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n")
	b.WriteString(fmt.Sprintf("%s<integer>%d</integer>\n", tabs, value))
}

func writePlistBool(b *strings.Builder, key string, value bool) {
	b.WriteString("\t<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n")
	if value {
		b.WriteString("\t<true/>\n")
	} else {
		b.WriteString("\t<false/>\n")
	}
}

func writeEscaped(w io.StringWriter, s string) {
	for _, r := range s {
		switch r {
		case '&':
			_, _ = w.WriteString("&amp;")
		case '<':
			_, _ = w.WriteString("&lt;")
		case '>':
			_, _ = w.WriteString("&gt;")
		case '"':
			_, _ = w.WriteString("&#34;")
		case '\'':
			_, _ = w.WriteString("&#39;")
		default:
			_, _ = w.WriteString(string(r))
		}
	}
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func bootstrapTask(task Task) error {
	return runLaunchctl("bootstrap", launchdDomain(), launchAgentPath(task.Label))
}

func bootoutTask(task Task) error {
	return runLaunchctl("bootout", launchdDomain()+"/"+task.Label)
}

func runLaunchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isLaunchctlNotLoaded(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such process") || strings.Contains(text, "could not find service") || strings.Contains(text, "not found")
}

func launchdDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchAgentPath(label string) string {
	if label == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func taskLogPath(task Task, stderr bool) string {
	if stderr && task.Stderr != "" {
		return task.Stderr
	}
	if !stderr && task.Stdout != "" {
		return task.Stdout
	}
	return task.Log
}

func taskEnv(task Task) []string {
	env := os.Environ()
	for k, v := range task.Env {
		env = append(env, k+"="+expandHome(v))
	}
	return env
}

func defaultLaunchdPath() string {
	return "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
}

func normalizeXML(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func checkExecutable(task Task, cfgPath string, issues *[]DoctorIssue) {
	exe := expandHome(task.Command[0])
	if strings.Contains(exe, "/") {
		if _, err := os.Stat(exe); err != nil {
			*issues = append(*issues, DoctorIssue{Level: "warn", Code: "command_missing", Message: "executable not found for task: " + task.Name, Ref: exe})
		}
		return
	}
	if _, err := exec.LookPath(exe); err != nil {
		*issues = append(*issues, DoctorIssue{Level: "warn", Code: "command_missing", Message: "executable not in PATH for task: " + task.Name, Ref: cfgPath})
	}
}

func loadConfig() (Config, string, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, path, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "autotask")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autotask")
}

func configPath() string {
	if path := os.Getenv("AUTOTASK_CONFIG"); path != "" {
		return path
	}
	return filepath.Join(configDir(), "tasks.yaml")
}

func printTasks(items []DiscoveredTask) {
	if len(items) == 0 {
		fmt.Println("No tasks found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SOURCE\tSTATUS\tLABEL\tSCHEDULE\tCOMMAND")
	for _, item := range items {
		status := item.Status
		if status == "" {
			status = "-"
		}
		if !item.Valid {
			status = "invalid"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", item.Source, status, item.Label, emptyDash(item.Schedule), truncate(item.Command, 90))
	}
	_ = w.Flush()
}

func printIssues(issues []DoctorIssue) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LEVEL\tCODE\tMESSAGE\tREF")
	for _, issue := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", issue.Level, issue.Code, issue.Message, issue.Ref)
	}
	_ = w.Flush()
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func sortTasks(items []DiscoveredTask) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		return items[i].Label < items[j].Label
	})
}

func sortIssues(issues []DoctorIssue) {
	order := map[string]int{"error": 0, "warn": 1, "info": 2}
	sort.Slice(issues, func(i, j int) bool {
		if order[issues[i].Level] != order[issues[j].Level] {
			return order[issues[i].Level] < order[issues[j].Level]
		}
		return issues[i].Code < issues[j].Code
	})
}

func filterPersonal(items []DiscoveredTask) []DiscoveredTask {
	filtered := make([]DiscoveredTask, 0, len(items))
	for _, item := range items {
		if item.Managed {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func launchdCommand(pl map[string]any) string {
	if args, ok := pl["ProgramArguments"].([]any); ok && len(args) > 0 {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, str(a))
		}
		return strings.Join(parts, " ")
	}
	return str(pl["Program"])
}

func launchdSchedule(pl map[string]any) string {
	var parts []string
	if sci, ok := pl["StartCalendarInterval"]; ok {
		parts = append(parts, "calendar "+formatCalendarValue(sci))
	}
	if si, ok := pl["StartInterval"]; ok {
		parts = append(parts, "every "+numberString(si)+"s")
	}
	if b, ok := pl["RunAtLoad"].(bool); ok && b {
		parts = append(parts, "run-at-load")
	}
	if _, ok := pl["KeepAlive"]; ok {
		parts = append(parts, "keep-alive")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

func formatCalendarValue(v any) string {
	switch x := v.(type) {
	case []any:
		var chunks []string
		for _, one := range x {
			chunks = append(chunks, formatCalendarValue(one))
		}
		return strings.Join(chunks, ",")
	case map[string]any:
		keys := []string{"Month", "Day", "Weekday", "Hour", "Minute"}
		var parts []string
		for _, k := range keys {
			if val, ok := x[k]; ok {
				parts = append(parts, strings.ToLower(k)+"="+numberString(val))
			}
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprint(v)
	}
}

func formatConfigSchedule(s Schedule) string {
	switch s.Type {
	case "cron":
		return s.Cron
	case "interval":
		return fmt.Sprintf("every %ds", s.EverySeconds)
	case "calendar":
		var parts []string
		if s.Month != nil {
			parts = append(parts, fmt.Sprintf("month=%d", *s.Month))
		}
		if s.Day != nil {
			parts = append(parts, fmt.Sprintf("day=%d", *s.Day))
		}
		if s.Weekday != nil {
			parts = append(parts, fmt.Sprintf("weekday=%d", *s.Weekday))
		}
		if s.Hour != nil {
			parts = append(parts, fmt.Sprintf("hour=%d", *s.Hour))
		}
		if s.Minute != nil {
			parts = append(parts, fmt.Sprintf("minute=%d", *s.Minute))
		}
		return strings.Join(parts, " ")
	default:
		return s.Type
	}
}

func commandSignature(command string) string {
	normalized := normalizedCommandKey(command)
	h := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(h[:])
}

func normalizedCommandKey(command string) string {
	if script := firstRegexpSubmatch(command, `(/[^ "';&|<>]+\.sh)`); script != "" {
		return script
	}
	return strings.Join(strings.Fields(command), " ")
}

func looksPersonal(label, command, path string) bool {
	all := strings.ToLower(label + " " + command + " " + path)
	home, _ := os.UserHomeDir()
	userLaunchAgent := strings.HasPrefix(path, filepath.Join(home, "Library", "LaunchAgents"))
	for _, p := range personalPrefixes() {
		if userLaunchAgent && strings.HasPrefix(strings.ToLower(label), p) {
			return true
		}
	}
	return strings.Contains(all, strings.ToLower(home+"/documents/project")) ||
		strings.Contains(all, strings.ToLower(home+"/src")) ||
		strings.Contains(all, strings.ToLower(home+"/.config/agentctl"))
}

func personalPrefixes() []string {
	raw := os.Getenv("AUTOTASK_PERSONAL_PREFIXES")
	if raw == "" {
		raw = "local.,autotask.,me."
	}
	var prefixes []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			prefixes = append(prefixes, part)
		}
	}
	return prefixes
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func firstArg(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func flagValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
		prefix := flag + "="
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func str(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func numberString(v any) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return fmt.Sprint(x)
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprint(x)
	}
}

func intValue(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}

func boolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	default:
		return false
	}
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func expandHome(s string) string {
	if s == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(s, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, s[2:])
	}
	return s
}

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }

func taskEnabled(task Task) bool {
	if task.Enabled == nil {
		return true
	}
	return *task.Enabled
}

func firstRegexpSubmatch(text, expr string) string {
	re := regexp.MustCompile(expr)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func scanCommand(args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
