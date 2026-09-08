package logsink

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriterReopenPreservesEvidenceAndRotatesAtLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openbts.log")
	w, err := NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	first := bytes.Repeat([]byte("a"), int(maxLogBytes))
	if n, err := w.Write(first); err != nil || n != len(first) {
		t.Fatalf("initial write: n=%d err=%v", n, err)
	}
	w.Close()
	w, err = NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if info, err := os.Stat(path); err != nil || info.Size() != maxLogBytes {
		t.Fatalf("reopening truncated evidence: info=%v err=%v", info, err)
	}
	if _, err := w.Write([]byte("new session\n")); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".1")
	if err != nil || !bytes.Equal(backup, first) {
		t.Fatalf("previous session was not rotated intact: %v", err)
	}
	// Oversized Write calls must still keep only bounded current + one backup.
	if _, err := w.Write(bytes.Repeat([]byte("b"), int(maxLogBytes)+1)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, path + ".1"} {
		info, err := os.Stat(name)
		if err != nil || info.Size() > maxLogBytes {
			t.Fatalf("unbounded retained log: path=%s info=%v err=%v", name, info, err)
		}
	}
	if _, err := os.Stat(path + ".2"); !os.IsNotExist(err) {
		t.Fatalf("unexpected second backup: %v", err)
	}
	w.Close()
	if _, err := w.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after close: %v", err)
	}
}

func TestWriterAdoptsOversizedHistoricalLogsWithBoundedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openbts.log")
	want := bytes.Repeat([]byte("e"), int(maxLogBytes))
	for _, name := range []string{path, path + ".1"} {
		if err := os.WriteFile(name, append([]byte("old discarded prefix"), want...), 0600); err != nil {
			t.Fatal(err)
		}
	}
	w, err := NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, name := range []string{path, path + ".1"} {
		data, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(data, want) {
			t.Fatalf("historical tail retention failed for %s: bytes=%d err=%v", name, len(data), err)
		}
	}
}
