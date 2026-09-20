package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
)

func readEvents(t *testing.T, path string) []Event {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("line %q is not valid JSON: %v", line, err)
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return events
}

func historyFiles(t *testing.T, repo string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(repo, "history"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("reading history dir: %v", err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// A run that changes nothing must leave nothing behind, or a nightly cron job
// fills the directory with empty files.
func TestNoFileWithoutEvents(t *testing.T) {
	repo := t.TempDir()

	log := Open(repo, "fetch")
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if names := historyFiles(t, repo); len(names) != 0 {
		t.Errorf("a run with no events left %v behind", names)
	}
	if p := log.Path(); p != "" {
		t.Errorf("Path() = %q, want empty", p)
	}
}

func TestRecordWritesJSONLines(t *testing.T) {
	repo := t.TempDir()

	log := Open(repo, "fetch")
	defer log.Close()

	events := []Event{
		{Event: KeyDiscovered, Key: "SHA256:aaa", KeyName: "id_rsa.pub"},
		{Event: BindingAdded, Key: "SHA256:aaa", Account: "root@host", Location: data.AUTHORIZED_KEYS},
		{Event: BindingRemoved, Key: "SHA256:bbb", Account: "root@host", Location: data.AUTHORIZED_KEYS},
	}
	for _, e := range events {
		if err := log.Record(e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	got := readEvents(t, log.Path())
	if len(got) != len(events) {
		t.Fatalf("wrote %d events, want %d", len(got), len(events))
	}

	for i := range events {
		if got[i].Event != events[i].Event || got[i].Key != events[i].Key {
			t.Errorf("event %d = %+v, want %+v", i, got[i], events[i])
		}
		if got[i].Time.IsZero() {
			t.Errorf("event %d has no timestamp", i)
		}
	}

	if got[1].Account != "root@host" || got[1].Location != data.AUTHORIZED_KEYS {
		t.Errorf("binding event lost its target: %+v", got[1])
	}
}

// The target lives in the record, not the filename, so "everything that
// happened to one host" stays a single grep.
func TestEventsAreGreppableByAccount(t *testing.T) {
	repo := t.TempDir()

	log := Open(repo, "fetch")
	defer log.Close()

	if err := log.Record(Event{Event: BindingRemoved, Key: "SHA256:aaa", Account: "root@prod-db-1"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	body, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.Contains(string(body), "root@prod-db-1") {
		t.Errorf("account not findable in the raw file:\n%s", body)
	}
}

// The runner's hostname is in the name because these files are merged through
// git from several machines; timestamp alone would collide.
func TestNameCarriesTimestampHostAndCommand(t *testing.T) {
	log := Open(t.TempDir(), "fetch")
	log.hostname = "build.example.com"
	log.started = time.Date(2026, 9, 20, 14, 32, 5, 0, time.UTC)

	got := log.Name()
	want := "20260920T143205Z_build_example_com_fetch.jsonl"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNameIsSortableByTime(t *testing.T) {
	early := Open(t.TempDir(), "fetch")
	early.hostname = "zzz"
	early.started = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	late := Open(t.TempDir(), "fetch")
	late.hostname = "aaa"
	late.started = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Timestamp first, so a listing reads chronologically across machines
	// rather than grouping by host.
	if !(early.Name() < late.Name()) {
		t.Errorf("%q should sort before %q", early.Name(), late.Name())
	}
}

// Two runs of the same command on one host in the same second must not
// overwrite each other.
func TestSameSecondRunsDoNotCollide(t *testing.T) {
	repo := t.TempDir()
	at := time.Date(2026, 9, 20, 14, 32, 5, 0, time.UTC)

	for i := 0; i < 3; i++ {
		log := Open(repo, "fetch")
		log.hostname = "host"
		log.started = at
		if err := log.Record(Event{Event: KeyDiscovered, Key: data.ID(string(rune('a' + i)))}); err != nil {
			t.Fatalf("Record: %v", err)
		}
		if err := log.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	names := historyFiles(t, repo)
	if len(names) != 3 {
		t.Fatalf("three runs in the same second produced %d files: %v", len(names), names)
	}

	seen := make(map[string]bool)
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate file name %q", n)
		}
		seen[n] = true
	}
}

// Ingestion runs concurrently with fetching, so Record is called from several
// goroutines at once.
func TestRecordIsConcurrencySafe(t *testing.T) {
	repo := t.TempDir()

	log := Open(repo, "fetch")
	defer log.Close()

	const writers, each = 8, 25

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if err := log.Record(Event{Event: KeyDiscovered, Key: "SHA256:aaa"}); err != nil {
					t.Errorf("Record: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	if got := readEvents(t, log.Path()); len(got) != writers*each {
		t.Errorf("wrote %d events, want %d -- interleaved writes were lost or corrupted", len(got), writers*each)
	}
}

// History is a record of work, not the work itself. A repository that cannot be
// written must not abort a fetch.
func TestRecordFailsSoftly(t *testing.T) {
	repo := t.TempDir()

	// A file where the history directory should be, so MkdirAll cannot succeed.
	if err := os.WriteFile(filepath.Join(repo, "history"), []byte("x"), 0600); err != nil {
		t.Fatalf("writing blocker: %v", err)
	}

	log := Open(repo, "fetch")
	defer log.Close()

	if err := log.Record(Event{Event: KeyDiscovered, Key: "SHA256:aaa"}); err == nil {
		t.Error("expected an error to be reported to the caller")
	}
	// Closing is still safe, and Path stays empty.
	if p := log.Path(); p != "" {
		t.Errorf("Path() = %q, want empty", p)
	}
}

// A nil log is a no-op, so callers need not branch on whether history is on.
func TestNilLogIsSafe(t *testing.T) {
	var log *Log

	if err := log.Record(Event{Event: KeyDiscovered}); err != nil {
		t.Errorf("Record on a nil log: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Errorf("Close on a nil log: %v", err)
	}
	if p := log.Path(); p != "" {
		t.Errorf("Path() = %q, want empty", p)
	}
}

func TestFileNameShape(t *testing.T) {
	shape := regexp.MustCompile(`^\d{8}T\d{6}Z_[\w]+_[\w]+(_\d+)?\.jsonl$`)

	repo := t.TempDir()
	log := Open(repo, "apply")
	if err := log.Record(Event{Event: AppliedAdd, Key: "SHA256:aaa", Account: "root@host"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	log.Close()

	names := historyFiles(t, repo)
	if len(names) != 1 {
		t.Fatalf("got %v, want one file", names)
	}
	if !shape.MatchString(names[0]) {
		t.Errorf("file name %q does not match the documented shape", names[0])
	}
}

// Path has to survive Close: a caller reporting what a run wrote asks at the
// end, not while writing.
func TestPathSurvivesClose(t *testing.T) {
	repo := t.TempDir()

	log := Open(repo, "fetch")
	if err := log.Record(Event{Event: KeyDiscovered, Key: "SHA256:aaa"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	during := log.Path()
	if during == "" {
		t.Fatal("Path() was empty while the log was open")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if after := log.Path(); after != during {
		t.Errorf("Path() = %q after Close, want %q", after, during)
	}
}
