package main

import (
	"strings"
	"testing"
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
