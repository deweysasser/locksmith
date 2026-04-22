package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns what was written.
// fmt.Println reads os.Stdout at call time, so this cleanly captures leveled output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}
	return buf.String()
}

// withLevel temporarily sets the package-global Level and restores it afterward.
func withLevel(t *testing.T, l OutputLevel, fn func()) {
	t.Helper()
	saved := Level
	Level = l
	defer func() { Level = saved }()
	fn()
}

func TestIsLevel(t *testing.T) {
	withLevel(t, NormalLevel, func() {
		if !IsLevel(NormalLevel) {
			t.Error("Normal should be visible at Normal level")
		}
		if !IsLevel(ErrorLevel) {
			t.Error("Error should always be visible at Normal level")
		}
		if IsLevel(VerboseLevel) {
			t.Error("Verbose should be gated at Normal level")
		}
		if IsLevel(DebugLevel) {
			t.Error("Debug should be gated at Normal level")
		}
	})
}

func TestLevelGating(t *testing.T) {
	tests := []struct {
		name       string
		level      OutputLevel
		want, hide []func(...interface{})
	}{
		{
			name:  "Normal hides Verbose and Debug",
			level: NormalLevel,
			want:  []func(...interface{}){Error, Warn, Normal},
			hide:  []func(...interface{}){Verbose, Debug},
		},
		{
			name:  "Verbose hides Debug",
			level: VerboseLevel,
			want:  []func(...interface{}){Error, Warn, Normal, Verbose},
			hide:  []func(...interface{}){Debug},
		},
		{
			name:  "Debug shows everything",
			level: DebugLevel,
			want:  []func(...interface{}){Error, Warn, Normal, Verbose, Debug},
			hide:  nil,
		},
		{
			name:  "Silent hides Normal and below-priority outputs",
			level: SilentLevel,
			want:  []func(...interface{}){Error, Warn},
			hide:  []func(...interface{}){Normal, Verbose, Debug},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withLevel(t, tc.level, func() {
				for i, fn := range tc.want {
					got := captureStdout(t, func() { fn("visible", i) })
					if !strings.Contains(got, "visible") {
						t.Errorf("want[%d] produced no output: %q", i, got)
					}
				}
				for i, fn := range tc.hide {
					got := captureStdout(t, func() { fn("hidden", i) })
					if got != "" {
						t.Errorf("hide[%d] leaked output: %q", i, got)
					}
				}
			})
		})
	}
}

func TestFormatted(t *testing.T) {
	withLevel(t, DebugLevel, func() {
		cases := []struct {
			name string
			fn   func(string, ...interface{})
		}{
			{"Errorf", Errorf},
			{"Warnf", Warnf},
			{"Normalf", Normalf},
			{"Verbosef", Verbosef},
			{"Debugf", Debugf},
		}
		for _, tc := range cases {
			got := captureStdout(t, func() { tc.fn("hello %s %d", "world", 42) })
			if !strings.Contains(got, "hello world 42") {
				t.Errorf("%s: want formatted output, got %q", tc.name, got)
			}
		}
	})
}

func TestFormattedGating(t *testing.T) {
	withLevel(t, NormalLevel, func() {
		got := captureStdout(t, func() { Debugf("secret=%d", 7) })
		if got != "" {
			t.Errorf("Debugf should be gated at Normal level, got %q", got)
		}
	})
}
