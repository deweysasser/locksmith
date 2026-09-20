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
		{"under a minute reads as current", 30 * time.Second, "right now"},
		{"exactly zero reads as current", 0, "right now"},
		{"a minute", time.Minute, "1m"},
		{"minutes", 30 * time.Minute, "30m"},
		{"just under an hour is still minutes", 59 * time.Minute, "59m"},
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

// A key whose Earliest timestamp is somehow in the future produces a negative
// age. Rendering that as "-5h" was worse than useless; it reads as current.
func TestFormatAgeNegative(t *testing.T) {
	if got := formatAge(-5 * time.Hour); got != "right now" {
		t.Errorf("formatAge(-5h) = %q, want %q", got, "right now")
	}
}

// Blank is reserved for "no date known" -- which is a real state, since several
// sources supply no creation date at all. A key that IS dated must never render
// as blank, however recently it was seen.
func TestFormatAgeNeverBlankForAKnownDate(t *testing.T) {
	for _, d := range []time.Duration{
		0, time.Second, time.Minute, time.Hour, 24 * time.Hour,
		7 * 24 * time.Hour, 365 * 24 * time.Hour, 3 * 365 * 24 * time.Hour,
	} {
		if got := formatAge(d); got == "" {
			t.Errorf("formatAge(%s) rendered blank, which means \"date unknown\"", d)
		}
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
