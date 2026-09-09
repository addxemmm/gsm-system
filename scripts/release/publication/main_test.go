package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "configs", "seeds")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name := range tables {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", "seeds", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCleanSeeds(t *testing.T) {
	root := fixture(t)
	output := filepath.Join(root, "clean")
	if err := cleanSeeds(root, output); err != nil {
		t.Fatal(err)
	}
	for name := range tables {
		data, err := query(filepath.Join(output, name), "PRAGMA freelist_count;", true)
		if err != nil || !strings.Contains(string(data), `"freelist_count":0`) {
			t.Fatal("dirty reconstructed seed")
		}
	}
}

func TestRejectSeedContamination(t *testing.T) {
	for _, tc := range []struct{ name, file, sql string }{
		{"business", "sqlite3_init.db", "INSERT INTO rates VALUES ('fixture',1);"},
		{"metadata", "TMSITable_init.db", "INSERT INTO ATTR_TABLE VALUES ('EXTRA','fixture');"},
		{"trigger", "sqlite3_init.db", "CREATE TRIGGER unexpected AFTER INSERT ON rates BEGIN SELECT 1; END;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			if _, err := query(filepath.Join(root, "configs", "seeds", tc.file), tc.sql, false); err != nil {
				t.Fatal(err)
			}
			if cleanSeeds(root, "") == nil {
				t.Fatal("contaminated seed accepted")
			}
		})
	}
}

func TestNoRawSeedInFinalStage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "docker", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "# ---------- stage 4: runtime ----------", 2)
	if len(parts) != 2 {
		t.Fatal("runtime stage missing")
	}
	runtime := parts[1]
	if strings.Contains(runtime, "COPY configs/ ") || strings.Contains(runtime, "COPY configs/seeds/TMSITable_init.db") ||
		!strings.Contains(runtime, "COPY --from=clean-seeds /clean-seeds/ /OpenBTS/") {
		t.Fatal("raw historical seed pages could enter runtime")
	}
}
