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
	if strings.Contains(got, " && ") {
		t.Fatalf("unescaped ampersand in plist:\n%s", got)
	}
	if !strings.Contains(got, "echo a &amp;&amp; echo b") {
		t.Fatalf("missing escaped command:\n%s", got)
	}
}

func TestCrontabCommand(t *testing.T) {
	got := crontabCommand("0 3 * * 0 /path/to/job.sh >> ~/job.log 2>&1")
	want := "/path/to/job.sh >> ~/job.log 2>&1"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}
