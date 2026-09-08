package gsm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	if err != nil || len(items) != 7 || items[5].ID != "a-first" || items[6].ID != "z-last" {
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

func TestDefaultPresetsExactAndFreshStoreIsVersion2(t *testing.T) {
	expectedIdentity := []struct{ id, name, band, c0, mcc, mnc string }{
		{"0", "addx", "1800", "540", "001", "01"},
		{"1", "ChinaMobile", "900", "55", "460", "00"},
		{"2", "ChinaMobile", "1800", "540", "460", "00"},
		{"3", "ChinaUnicom", "900", "70", "460", "01"},
		{"4", "ChinaUnicom", "1800", "668", "460", "01"},
	}
	expected := make([]Preset, 0, len(expectedIdentity))
	for _, want := range expectedIdentity {
		expected = append(expected, Preset{
			ID: want.id, Name: want.name,
			Params: StartParams{
				ARFCNs: "1", C0: want.c0, Band: want.band, MCC: want.mcc, MNC: want.mnc,
				LAC: "1", CI: "1", ShortName: want.name, Network: "eth0",
			},
		})
	}
	gotDefaults := DefaultPresets()
	if !reflect.DeepEqual(gotDefaults, expected) {
		t.Fatalf("defaults mismatch\n got=%+v\nwant=%+v", gotDefaults, expected)
	}
	dir := t.TempDir()
	store := NewPresetStore(dir)
	items, err := store.List()
	if err != nil || !reflect.DeepEqual(items, expected) {
		t.Fatalf("fresh items=%+v err=%v", items, err)
	}
	disk := readPresetFile(t, filepath.Join(dir, "presets.json"))
	if disk.Version != 2 || !reflect.DeepEqual(disk.Items, expected) {
		t.Fatalf("fresh disk=%+v", disk)
	}
}

func TestPresetStoreMigratesVersion1AndPreservesConflictingUserID(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "presets.json")
		writePresetFile(t, path, presetFile{Version: 1, Items: []Preset{}})
		store := NewPresetStore(dir)
		items, err := store.List()
		if err != nil || !reflect.DeepEqual(items, DefaultPresets()) {
			t.Fatalf("items=%+v err=%v", items, err)
		}
		if got := readPresetFile(t, path).Version; got != 2 {
			t.Fatalf("version=%d", got)
		}
	})

	t.Run("custom and conflict", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "presets.json")
		userZero := testPreset("0")
		userZero.Name, userZero.Params.ShortName = "User Zero", "UserZero"
		custom := testPreset("custom")
		writePresetFile(t, path, presetFile{Version: 1, Items: []Preset{custom, userZero}})
		store := NewPresetStore(dir)
		got, err := store.Get("0")
		if err != nil || got.Name != "User Zero" || got.Params.ShortName != "UserZero" {
			t.Fatalf("conflicting ID overwritten: %+v err=%v", got, err)
		}
		items, err := store.List()
		if err != nil || len(items) != 6 {
			t.Fatalf("migration items=%d err=%v", len(items), err)
		}
		// Reopening version 2 must neither duplicate nor restore anything.
		again := NewPresetStore(dir)
		againItems, err := again.List()
		if err != nil || !reflect.DeepEqual(againItems, items) {
			t.Fatalf("second restart changed items: %+v err=%v", againItems, err)
		}
	})
}

func TestPresetStoreVersion2DeletionDoesNotRespawnDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewPresetStore(dir)
	edited, err := store.Get("1")
	if err != nil {
		t.Fatal(err)
	}
	edited.Name, edited.Params.ShortName = "Edited Mobile", "EditedMobile"
	if err := store.Update("1", edited); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("2"); err != nil {
		t.Fatal(err)
	}
	reloaded := NewPresetStore(dir)
	if _, err := reloaded.Get("2"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("deleted default respawned: %v", err)
	}
	gotEdited, err := reloaded.Get("1")
	if err != nil || gotEdited.Name != "Edited Mobile" || gotEdited.Params.ShortName != "EditedMobile" {
		t.Fatalf("edited default reset: %+v err=%v", gotEdited, err)
	}
	items, err := reloaded.List()
	if err != nil || len(items) != 4 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
}

func TestPresetStoreVersion1MigrationCapacityFailurePreservesSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")
	items := make([]Preset, 0, maxPresets)
	for i := 0; i < maxPresets; i++ {
		items = append(items, testPreset(fmt.Sprintf("u%03d", i)))
	}
	writePresetFile(t, path, presetFile{Version: 1, Items: items})
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := NewPresetStore(dir)
	if _, err := store.List(); !errors.Is(err, ErrPresetStore) {
		t.Fatalf("capacity error=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("capacity failure overwrote source: err=%v", err)
	}
}

func TestPresetStoreVersion1MigrationAtExactCapacity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")
	items := make([]Preset, 0, maxPresets-len(DefaultPresets()))
	for i := 0; i < cap(items); i++ {
		items = append(items, testPreset(fmt.Sprintf("u%03d", i)))
	}
	writePresetFile(t, path, presetFile{Version: 1, Items: items})
	store := NewPresetStore(dir)
	got, err := store.List()
	if err != nil || len(got) != maxPresets {
		t.Fatalf("exact-capacity migration items=%d err=%v", len(got), err)
	}
	if version := readPresetFile(t, path).Version; version != 2 {
		t.Fatalf("version=%d", version)
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

func TestPresetStoreRejectsAmbiguousOrMissingItemsWithoutOverwrite(t *testing.T) {
	valid, err := json.Marshal(testPreset("user"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"duplicate items":      []byte(`{"version":1,"items":[` + string(valid) + `],"items":[]}`),
		"case duplicate items": []byte(`{"version":1,"items":[` + string(valid) + `],"Items":[]}`),
		"missing items":        []byte(`{"version":1}`),
		"null items":           []byte(`{"version":1,"items":null}`),
		"nested duplicate":     []byte(`{"version":1,"items":[{"id":"user","id":"lost","name":"x","description":"","params":{}}]}`),
	}
	for name, original := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "presets.json")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewPresetStore(dir)
			if _, err := store.List(); !errors.Is(err, ErrPresetStore) {
				t.Fatalf("load error=%v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(original) {
				t.Fatalf("corrupt source changed: err=%v", err)
			}
		})
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
	if err != nil || len(items) != count/2+len(DefaultPresets()) {
		t.Fatalf("reload count=%d err=%v", len(items), err)
	}
	for _, preset := range items {
		if len(preset.ID) > 1 && preset.ID[:2] == "p-" && preset.Name != "updated "+preset.ID {
			t.Fatalf("partial or stale item: %+v", preset)
		}
	}
}

func writePresetFile(t *testing.T, path string, disk presetFile) {
	t.Helper()
	b, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readPresetFile(t *testing.T, path string) presetFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var disk presetFile
	if err := json.Unmarshal(b, &disk); err != nil {
		t.Fatal(err)
	}
	return disk
}
