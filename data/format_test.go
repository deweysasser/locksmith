package data

import (
	"strings"
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	const hour = time.Hour
	const week = 7 * 24 * hour
	const year = 365 * 24 * hour

	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{"less than an hour reads as nothing", 30 * time.Minute, ""},
		{"exactly zero reads as nothing", 0, ""},
		{"an hour", hour, "1h"},
		{"hours up to and including a week stay in hours", week, "168h"},
		{"just past a week rolls over to weeks", week + hour, "1w"},
		{"just under a year is still weeks", year - hour, "52w"},
		{"exactly a year", year, "1y"},
		{"two years with no remainder", 2 * year, "2y"},
		{"a year and a week pads the week count", year + week, "1y01w"},
		{"a year and ten weeks", year + 10*week, "1y10w"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatAge(tc.age); got != tc.want {
				t.Errorf("formatAge(%s) = %q, want %q", tc.age, got, tc.want)
			}
		})
	}
}

// A key whose Earliest timestamp is in the future produces a negative age. It
// should not panic or render something absurd; hours simply go negative.
func TestFormatAgeNegative(t *testing.T) {
	if got := formatAge(-5 * time.Hour); got != "-5h" {
		t.Errorf("formatAge(-5h) = %q, want %q", got, "-5h")
	}
}

func TestStandardStringShowsExpiry(t *testing.T) {
	key := keyImpl{Type: "SSHKey", Deprecated: true}
	got := key.StandardString("id1")

	if !strings.Contains(got, "*EX*") {
		t.Errorf("deprecated key should be marked *EX*, got %q", got)
	}
}

func TestStandardStringShowsAgeWhenNotDeprecated(t *testing.T) {
	key := keyImpl{Type: "SSHKey", Earliest: time.Now().Add(-3 * time.Hour)}
	got := key.StandardString("id1")

	if !strings.Contains(got, "3h") {
		t.Errorf("key aged 3h should render its age, got %q", got)
	}
}

// Expiry takes priority over age: a deprecated key shows *EX*, not its age.
func TestStandardStringExpiryBeatsAge(t *testing.T) {
	key := keyImpl{Type: "SSHKey", Deprecated: true, Earliest: time.Now().Add(-3 * time.Hour)}
	got := key.StandardString("id1")

	if strings.Contains(got, "3h") {
		t.Errorf("deprecated key should not render an age, got %q", got)
	}
}

// Long IDs are truncated to 25 columns so list output stays aligned.
func TestStandardStringTruncatesLongIds(t *testing.T) {
	long := ID(strings.Repeat("x", 60))
	key := keyImpl{Type: "SSHKey"}
	got := key.StandardString(long)

	if strings.Contains(got, string(long)) {
		t.Errorf("long ID should be truncated, got %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Errorf("truncated ID should be marked with an ellipsis, got %q", got)
	}
}

func TestStandardStringKeepsShortIds(t *testing.T) {
	key := keyImpl{Type: "AWSKey"}
	got := key.StandardString("AKIASHORT")

	if !strings.Contains(got, "AKIASHORT") {
		t.Errorf("short ID should be rendered whole, got %q", got)
	}
}
