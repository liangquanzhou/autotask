package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseDisplayScheduleCalendar(t *testing.T) {
	got := parseDisplaySchedule("calendar weekday=0 hour=21 minute=30")
	if got.Type != "calendar" {
		t.Fatalf("type = %q", got.Type)
	}
	if got.Weekday == nil || *got.Weekday != 0 {
		t.Fatalf("weekday = %#v", got.Weekday)
	}
	if got.Hour == nil || *got.Hour != 21 {
		t.Fatalf("hour = %#v", got.Hour)
	}
	if got.Minute == nil || *got.Minute != 30 {
		t.Fatalf("minute = %#v", got.Minute)
	}
}

func TestRenderLaunchdPlistEscapesShellOperators(t *testing.T) {
	task := Task{
		Name:     "demo",
		Label:    "com.example.demo",
		Kind:     "launchd",
		Command:  []string{"/bin/zsh", "-lc", "echo a && echo b"},
		Schedule: Schedule{Type: "daemon"},
	}
	got, err := renderLaunchdPlist(task)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, " && ") {
		t.Fatalf("unescaped ampersand in plist:\n%s", got)
	}
	if !strings.Contains(got, "echo a &amp;&amp; echo b") {
		t.Fatalf("missing escaped command:\n%s", got)
	}
}

func TestRenderLaunchdPlistWrapsScheduledTasks(t *testing.T) {
	task := Task{
		Name:    "demo",
		Label:   "com.example.demo",
		Kind:    "launchd",
		Command: []string{"/bin/zsh", "-lc", "echo a && echo b"},
		Schedule: Schedule{
			Type: "calendar", Hour: intPtr(3), Minute: intPtr(0),
		},
	}
	got, err := renderLaunchdPlist(task)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<string>exec</string>") || !strings.Contains(got, "<string>demo</string>") {
		t.Fatalf("missing autotask exec wrapper:\n%s", got)
	}
	if strings.Contains(got, "echo a") {
		t.Fatalf("wrapped plist should not contain original shell command:\n%s", got)
	}
}

func TestCrontabCommand(t *testing.T) {
	got := crontabCommand("0 3 * * 0 /path/to/job.sh >> ~/job.log 2>&1")
	want := "/path/to/job.sh >> ~/job.log 2>&1"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestBuildUIStateHasTaskAliasAndActionableDiff(t *testing.T) {
	state := buildUIState(
		[]StatusRow{{Name: "demo", Label: "local.demo"}},
		DiffResult{Actions: []DiffAction{
			{Action: "noop"},
			{Action: "write", Task: "demo", Label: "local.demo"},
		}},
	)

	tasks, ok := state["tasks"].([]StatusRow)
	if !ok || len(tasks) != 1 || tasks[0].Name != "demo" {
		t.Fatalf("tasks = %#v", state["tasks"])
	}
	diff, ok := state["diff"].([]DiffAction)
	if !ok || len(diff) != 1 || diff[0].Action != "write" {
		t.Fatalf("diff = %#v", state["diff"])
	}
}

func TestBuildUIState_MergesRuntimeFailures(t *testing.T) {
	rfc := func(tm time.Time) string { return tm.Format(time.RFC3339) }
	nowRec := &RunRecord{ExitCode: 1, EndedAt: rfc(time.Now()), Success: false}
	oldRec := &RunRecord{ExitCode: 1, EndedAt: rfc(time.Now().Add(-10 * 24 * time.Hour)), Success: false}
	okRec := &RunRecord{ExitCode: 0, EndedAt: rfc(time.Now()), Success: true}
	garbageRec := &RunRecord{ExitCode: 1, EndedAt: "not-a-date", Success: false}

	status := []StatusRow{
		{Name: "failing-enabled", Enabled: true, Runs: RunInfo{Last: nowRec}},
		{Name: "succeeded", Enabled: true, Runs: RunInfo{Last: okRec}},
		{Name: "never-ran", Enabled: true},
		{Name: "failing-old", Enabled: true, Runs: RunInfo{Last: oldRec}},
		{Name: "failing-disabled", Enabled: false, Runs: RunInfo{Last: nowRec}},
		{Name: "failing-garbage-time", Enabled: true, Runs: RunInfo{Last: garbageRec}},
	}
	diff := DiffResult{Issues: []DoctorIssue{{Level: "error", Code: "config_drift", Message: "sync needed"}}}

	state := buildUIState(status, diff)

	issues, ok := state["issues"].([]DoctorIssue)
	if !ok {
		t.Fatalf("issues type = %T", state["issues"])
	}
	byRef := map[string]DoctorIssue{}
	runtimeCount := 0
	for _, iss := range issues {
		if iss.Code == "runtime_failure" {
			runtimeCount++
			byRef[iss.Ref] = iss
		}
	}
	if runtimeCount != 2 {
		t.Fatalf("runtime_failure count = %d, want 2; issues = %#v", runtimeCount, issues)
	}
	if _, ok := byRef["failing-enabled"]; !ok {
		t.Fatalf("missing issue for failing-enabled: %#v", issues)
	}
	if iss, ok := byRef["failing-enabled"]; ok {
		if iss.Level != "warn" {
			t.Fatalf("failing-enabled level = %q, want warn", iss.Level)
		}
		if !strings.Contains(iss.Message, "exit=1") || !strings.Contains(iss.Message, "ago") {
			t.Fatalf("failing-enabled message = %q", iss.Message)
		}
	}
	if iss, ok := byRef["failing-garbage-time"]; !ok {
		t.Fatalf("missing issue for failing-garbage-time (parse-fail should still emit): %#v", issues)
	} else if strings.Contains(iss.Message, "ago") {
		t.Fatalf("failing-garbage-time message should omit 'ago': %q", iss.Message)
	}
	if _, ok := byRef["failing-old"]; ok {
		t.Fatalf("failing-old should be filtered by 7d window: %#v", issues)
	}
	if _, ok := byRef["failing-disabled"]; ok {
		t.Fatalf("failing-disabled should be skipped: %#v", issues)
	}
	if _, ok := byRef["succeeded"]; ok {
		t.Fatalf("succeeded should not emit issue")
	}
	if _, ok := byRef["never-ran"]; ok {
		t.Fatalf("never-ran should not emit issue")
	}
	if len(issues) != 3 {
		t.Fatalf("total issues = %d, want 3 (1 config + 2 runtime); got %#v", len(issues), issues)
	}

	summary, ok := state["summary"].(map[string]int)
	if !ok {
		t.Fatalf("summary type = %T", state["summary"])
	}
	if summary["issues"] != 3 {
		t.Fatalf("summary.issues = %d, want 3", summary["issues"])
	}
}

func TestHumanTimeAgo(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h"},
		{3 * time.Hour, "3h"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range cases {
		if got := humanTimeAgo(tt.in); got != tt.want {
			t.Fatalf("humanTimeAgo(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHumanSchedule(t *testing.T) {
	cases := []struct {
		name string
		in   Schedule
		want string
	}{
		{name: "daemon", in: Schedule{Type: "daemon"}, want: "daemon"},
		{name: "interval", in: Schedule{Type: "interval", EverySeconds: 1800}, want: "every 30m"},
		{name: "daily", in: Schedule{Type: "calendar", Hour: intPtr(9), Minute: intPtr(30)}, want: "daily 09:30"},
		{name: "weekly", in: Schedule{Type: "calendar", Weekday: intPtr(6), Hour: intPtr(3), Minute: intPtr(0)}, want: "Sat 03:00"},
		{name: "monthly", in: Schedule{Type: "calendar", Day: intPtr(1), Hour: intPtr(19), Minute: intPtr(0)}, want: "monthly day 1 19:00"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanSchedule(tt.in); got != tt.want {
				t.Fatalf("humanSchedule() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanDisplaySchedule(t *testing.T) {
	got := humanDisplaySchedule("calendar weekday=0 hour=10 minute=0")
	if got != "Sun 10:00" {
		t.Fatalf("humanDisplaySchedule() = %q, want %q", got, "Sun 10:00")
	}
}
