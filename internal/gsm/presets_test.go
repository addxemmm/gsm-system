package gsm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func testPreset(id string) Preset {
	return Preset{
		ID: id, Name: "Lab " + id, Description: "test preset",
		Params: StartParams{ARFCNs: "1", C0: "55", Band: "900", MCC: "001", MNC: "01", LAC: "1", CI: "0", ShortName: "lab", Network: "eth0"},
	}
}

func TestPresetStoreCRUDPersistenceAndMode(t *testing.T) {
	dir := t.TempDir()
	store := NewPresetStore(dir)
	if err := store.Create(testPreset("z-last")); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(testPreset("a-first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(testPreset("a-first")); !errors.Is(err, ErrPresetExists) {
		t.Fatalf("duplicate error=%v", err)
	}
	items, err := store.List()
	if err != nil || len(items) != 2 || items[0].ID != "a-first" || items[1].ID != "z-last" {
		t.Fatalf("list=%+v err=%v", items, err)
	}
	updated := testPreset("ignored")
	updated.Name = "Updated"
	if err := store.Update("a-first", updated); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("z-last"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("missing"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("missing delete error=%v", err)
	}

	reloaded := NewPresetStore(dir)
	got, err := reloaded.Get("a-first")
	if err != nil || got.Name != "Updated" || got.ID != "a-first" {
		t.Fatalf("reloaded=%+v err=%v", got, err)
	}
	if _, err := reloaded.Get("z-last"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("deleted preset survived: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "presets.json"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%o", info.Mode().Perm())
		}
	}
}

func TestPresetStoreCorruptionFailsClosedWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")
	original := []byte(`{"version":1,"items":[`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewPresetStore(dir)
	if _, err := store.List(); !errors.Is(err, ErrPresetStore) {
		t.Fatalf("list error=%v", err)
	}
	if err := store.Create(testPreset("must-not-write")); !errors.Is(err, ErrPresetStore) {
		t.Fatalf("create error=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("corrupt file overwritten: %q", after)
	}
}

func TestPresetStoreDirectorySyncFailurePoisonsCommittedStore(t *testing.T) {
	dir := t.TempDir()
	store := NewPresetStore(dir)
	store.syncDir = func(string) error { return errors.New("injected sync failure") }
	if err := store.Create(testPreset("committed")); !errors.Is(err, ErrPresetStore) {
		t.Fatalf("create error=%v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "presets.json"))
	if err != nil {
		t.Fatalf("rename should already be committed: %v", err)
	}
	if err := store.Create(testPreset("must-not-follow")); !errors.Is(err, ErrPresetStore) {
		t.Fatalf("poisoned create error=%v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "presets.json"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("poisoned store overwrote committed bytes: err=%v", err)
	}
	reloaded := NewPresetStore(dir)
	if _, err := reloaded.Get("committed"); err != nil {
		t.Fatalf("committed file was not valid: %v", err)
	}
	if _, err := reloaded.Get("must-not-follow"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("post-failure mutation reached disk: %v", err)
	}
}

func TestPresetStoreRejectsInvalidAndOversizedData(t *testing.T) {
	store := NewPresetStore(t.TempDir())
	bad := testPreset("Bad ID")
	if err := store.Create(bad); !errors.Is(err, ErrPresetInvalid) {
		t.Fatalf("bad id error=%v", err)
	}
	bad = testPreset("bad-band")
	bad.Params.Band = "850"
	if err := store.Create(bad); !errors.Is(err, ErrPresetInvalid) {
		t.Fatalf("bad params error=%v", err)
	}
	bad = testPreset("too-long")
	bad.Description = string(make([]byte, maxDescriptionBytes+1))
	if err := store.Create(bad); !errors.Is(err, ErrPresetInvalid) {
		t.Fatalf("long description error=%v", err)
	}
}

func TestPresetStoreConcurrentOperationsRemainConsistent(t *testing.T) {
	dir := t.TempDir()
	store := NewPresetStore(dir)
	const count = 64
	var wg sync.WaitGroup
	errs := make(chan error, count*3)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- store.Create(testPreset(fmt.Sprintf("p-%02d", i)))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	errs = make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("p-%02d", i)
			if i%2 == 0 {
				errs <- store.Delete(id)
				return
			}
			preset := testPreset(id)
			preset.Name = "updated " + id
			errs <- store.Update(id, preset)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	reloaded := NewPresetStore(dir)
	items, err := reloaded.List()
	if err != nil || len(items) != count/2 {
		t.Fatalf("reload count=%d err=%v", len(items), err)
	}
	for _, preset := range items {
		if preset.Name != "updated "+preset.ID {
			t.Fatalf("partial or stale item: %+v", preset)
		}
	}
}
