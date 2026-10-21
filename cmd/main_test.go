package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kardianos/service"
)

func init() {
	sl = service.ConsoleLogger
}

func thisYear() string {
	return strconv.Itoa(time.Now().Year() - 2000)
}

// chdirTemp runs the test in an empty directory, where the counter files live.
func chdirTemp(t *testing.T) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestCloseClientWithoutConnection(t *testing.T) {
	task := Task{_name: "t"}
	closeClient(&task)
	if task._client != nil {
		t.Fatal("client is still set")
	}
}

func TestNextRtKeyStartsWithoutCounterFile(t *testing.T) {
	chdirTemp(t)
	key, err := getNextRtKey(Task{_name: "ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if want := thisYear() + "-1"; key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}

func TestNextRtKeyCountsUp(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("ticket.cnt", []byte(thisYear()+"-41\n"), 0644); err != nil {
		t.Fatal(err)
	}
	key, err := getNextRtKey(Task{_name: "ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if want := thisYear() + "-42"; key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}

func TestNextRtKeyRestartsInNewYear(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("ticket.cnt", []byte("19-41"), 0644); err != nil {
		t.Fatal(err)
	}
	key, err := getNextRtKey(Task{_name: "ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if want := thisYear() + "-1"; key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}

func TestNextRtKeyRejectsMalformedCounter(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("ticket.cnt", []byte("26"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := getNextRtKey(Task{_name: "ticket"}); err == nil {
		t.Fatal("expected an error for a counter without '-'")
	}
}

func TestJsonnetDumpWritesStdout(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.jsonnet")
	src := `{imapAccounts: {}, smtpAccounts: {}, tasks: {}}`
	if err := os.WriteFile(cfg, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	*jsonetDump = true
	t.Cleanup(func() { *jsonetDump = false })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	_, cfgErr := readConfig(cfg)
	os.Stdout = stdout
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if cfgErr != nil {
		t.Fatal(cfgErr)
	}
	if !strings.Contains(string(out), `"imapAccounts"`) {
		t.Fatalf("stdout has no JSON: %q", out)
	}
}
