package gsm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxPresetsFileBytes = 1 << 20
	maxPresets          = 256
	maxPresetNameBytes  = 128
	maxDescriptionBytes = 2048
)

var (
	ErrPresetNotFound = errors.New("preset not found")
	ErrPresetExists   = errors.New("preset already exists")
	ErrPresetInvalid  = errors.New("invalid preset")
	ErrPresetStore    = errors.New("preset store unavailable")
	presetIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
)

// Preset is a named, reusable, complete cell configuration.
type Preset struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Params      StartParams `json:"params"`
}

type presetFile struct {
	Version int      `json:"version"`
	Items   []Preset `json:"items"`
}

// PresetStore serializes access to the atomically persisted preset file.
// A malformed file poisons the store for its lifetime so a later mutation can
// never replace data that the process was unable to understand.
type PresetStore struct {
	mu      sync.RWMutex
	path    string
	items   map[string]Preset
	loadErr error
	syncDir func(string) error
}

func NewPresetStore(dataDir string) *PresetStore {
	s := &PresetStore{
		path: filepath.Join(dataDir, "presets.json"), items: make(map[string]Preset),
		syncDir: syncProfileDirectory,
	}
	s.loadErr = s.load()
	return s
}

func ValidPresetID(id string) bool { return presetIDPattern.MatchString(id) }

func ValidatePreset(p Preset) error {
	if !ValidPresetID(p.ID) {
		return fmt.Errorf("%w: id must match [a-z0-9][a-z0-9_-]{0,63}", ErrPresetInvalid)
	}
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > maxPresetNameBytes {
		return fmt.Errorf("%w: name must contain 1-%d bytes", ErrPresetInvalid, maxPresetNameBytes)
	}
	if len(p.Description) > maxDescriptionBytes {
		return fmt.Errorf("%w: description must contain at most %d bytes", ErrPresetInvalid, maxDescriptionBytes)
	}
	if err := p.Params.Validate(); err != nil {
		return fmt.Errorf("%w: params.%v", ErrPresetInvalid, err)
	}
	return nil
}

func (s *PresetStore) Path() string { return s.path }

func (s *PresetStore) List() ([]Preset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	items := make([]Preset, 0, len(s.items))
	for _, preset := range s.items {
		items = append(items, preset)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (s *PresetStore) Get(id string) (Preset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.loadErr != nil {
		return Preset{}, s.loadErr
	}
	preset, ok := s.items[id]
	if !ok {
		return Preset{}, ErrPresetNotFound
	}
	return preset, nil
}

func (s *PresetStore) Create(p Preset) error {
	if err := ValidatePreset(p); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return s.loadErr
	}
	if _, ok := s.items[p.ID]; ok {
		return ErrPresetExists
	}
	if len(s.items) >= maxPresets {
		return fmt.Errorf("%w: at most %d presets are allowed", ErrPresetInvalid, maxPresets)
	}
	next := clonePresets(s.items)
	next[p.ID] = p
	if err := s.persist(next); err != nil {
		return err
	}
	s.items = next
	return nil
}

func (s *PresetStore) Update(id string, p Preset) error {
	p.ID = id
	if err := ValidatePreset(p); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return s.loadErr
	}
	if _, ok := s.items[id]; !ok {
		return ErrPresetNotFound
	}
	next := clonePresets(s.items)
	next[id] = p
	if err := s.persist(next); err != nil {
		return err
	}
	s.items = next
	return nil
}

func (s *PresetStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return s.loadErr
	}
	if _, ok := s.items[id]; !ok {
		return ErrPresetNotFound
	}
	next := clonePresets(s.items)
	delete(next, id)
	if err := s.persist(next); err != nil {
		return err
	}
	s.items = next
	return nil
}

func clonePresets(items map[string]Preset) map[string]Preset {
	next := make(map[string]Preset, len(items))
	for id, preset := range items {
		next[id] = preset
	}
	return next
}

func (s *PresetStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		items := make(map[string]Preset, len(DefaultPresets()))
		for _, preset := range DefaultPresets() {
			items[preset.ID] = preset
		}
		if err := s.persist(items); err != nil {
			return fmt.Errorf("%w: initialize defaults: %v", ErrPresetStore, err)
		}
		s.items = items
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: open: %v", ErrPresetStore, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: stat: %v", ErrPresetStore, err)
	}
	if info.Size() > maxPresetsFileBytes {
		return fmt.Errorf("%w: presets.json exceeds %d bytes", ErrPresetStore, maxPresetsFileBytes)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: presets.json is not a regular file", ErrPresetStore)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxPresetsFileBytes+1))
	if err != nil {
		return fmt.Errorf("%w: read: %v", ErrPresetStore, err)
	}
	if len(b) > maxPresetsFileBytes {
		return fmt.Errorf("%w: presets.json exceeds %d bytes", ErrPresetStore, maxPresetsFileBytes)
	}
	// Close the version-1 source before an atomic migration rename. In
	// particular, Windows may reject replacement while this handle is open.
	if err := f.Close(); err != nil {
		return fmt.Errorf("%w: close after read: %v", ErrPresetStore, err)
	}
	if err := rejectDuplicatePresetJSONNames(b); err != nil {
		return fmt.Errorf("%w: corrupt presets.json: %v", ErrPresetStore, err)
	}
	var required map[string]json.RawMessage
	if err := json.Unmarshal(b, &required); err != nil {
		return fmt.Errorf("%w: corrupt presets.json: %v", ErrPresetStore, err)
	}
	var itemsJSON json.RawMessage
	var hasItems bool
	for name, value := range required {
		if strings.EqualFold(name, "items") {
			itemsJSON, hasItems = value, true
			break
		}
	}
	if !hasItems || bytes.Equal(bytes.TrimSpace(itemsJSON), []byte("null")) {
		return fmt.Errorf("%w: corrupt presets.json: items array is required", ErrPresetStore)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var disk presetFile
	if err := decoder.Decode(&disk); err != nil {
		return fmt.Errorf("%w: corrupt presets.json: %v", ErrPresetStore, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%w: corrupt presets.json: trailing data", ErrPresetStore)
	}
	if (disk.Version != 1 && disk.Version != 2) || len(disk.Items) > maxPresets {
		return fmt.Errorf("%w: unsupported version or item count", ErrPresetStore)
	}
	loaded := make(map[string]Preset, len(disk.Items))
	for _, preset := range disk.Items {
		if err := ValidatePreset(preset); err != nil {
			return fmt.Errorf("%w: corrupt presets.json: %v", ErrPresetStore, err)
		}
		if _, duplicate := loaded[preset.ID]; duplicate {
			return fmt.Errorf("%w: corrupt presets.json: duplicate id %q", ErrPresetStore, preset.ID)
		}
		loaded[preset.ID] = preset
	}
	if disk.Version == 1 {
		next := clonePresets(loaded)
		for _, preset := range DefaultPresets() {
			if _, exists := next[preset.ID]; !exists {
				next[preset.ID] = preset
			}
		}
		if len(next) > maxPresets {
			return fmt.Errorf("%w: version-1 migration needs %d items, maximum is %d", ErrPresetStore, len(next), maxPresets)
		}
		if err := s.persist(next); err != nil {
			return fmt.Errorf("%w: migrate version 1: %v", ErrPresetStore, err)
		}
		loaded = next
	}
	s.items = loaded
	return nil
}

func rejectDuplicatePresetJSONNames(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, isDelim := token.(json.Delim)
		if !isDelim {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				nameToken, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := nameToken.(string)
				if !ok {
					return errors.New("JSON object name is not a string")
				}
				canonicalName := strings.ToLower(name)
				if _, duplicate := seen[canonicalName]; duplicate {
					return fmt.Errorf("duplicate JSON name %q", name)
				}
				seen[canonicalName] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return walk()
}

func (s *PresetStore) persist(items map[string]Preset) error {
	ordered := make([]Preset, 0, len(items))
	for _, preset := range items {
		ordered = append(ordered, preset)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	b, err := json.MarshalIndent(presetFile{Version: 2, Items: ordered}, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrPresetStore, err)
	}
	if len(b)+1 > maxPresetsFileBytes {
		return fmt.Errorf("%w: serialized data exceeds %d bytes", ErrPresetInvalid, maxPresetsFileBytes)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: mkdir: %v", ErrPresetStore, err)
	}
	f, err := os.CreateTemp(dir, ".presets-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: create temp: %v", ErrPresetStore, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	fail := func(operation string, err error) error {
		_ = f.Close()
		return fmt.Errorf("%w: %s: %v", ErrPresetStore, operation, err)
	}
	if err := f.Chmod(0o600); err != nil {
		return fail("chmod", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fail("write", err)
	}
	if err := f.Sync(); err != nil {
		return fail("sync", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%w: close: %v", ErrPresetStore, err)
	}
	var renameErr error
	for attempt := 0; attempt < 6; attempt++ {
		renameErr = os.Rename(tmp, s.path)
		if renameErr == nil {
			break
		}
		// Windows file scanners and sync clients can briefly hold the old
		// destination. Retrying the same atomic rename remains fail-safe.
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	if renameErr != nil {
		return fmt.Errorf("%w: rename: %v", ErrPresetStore, renameErr)
	}
	if err := s.syncDir(dir); err != nil {
		// Rename has committed the new file already. Poison this in-memory store
		// rather than risk overwriting committed bytes from stale state later.
		s.loadErr = fmt.Errorf("%w: sync directory after commit: %v", ErrPresetStore, err)
		return s.loadErr
	}
	return nil
}
