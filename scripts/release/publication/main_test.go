package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Model checkout from Git blobs, not autocrlf-transformed local source bytes.
// 从 Git 原始 blob 构造 fresh checkout 样本，避免依赖本地 autocrlf 字节。
func checkoutFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "..")
	if _, err := os.Stat(filepath.Join(source, ".git")); os.IsNotExist(err) {
		t.Skip("Git metadata excluded from corresponding-source archive")
	}
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", "HEAD", "configs", "firmware/uhd", "gsmsystem/asterisk")
	cmd.Dir = source
	paths, err := cmd.Output()
	if err != nil {
		t.Fatal("read committed publication input list")
	}
	root := t.TempDir()
	for _, path := range strings.Split(strings.TrimSpace(string(paths)), "\n") {
		if path == "" {
			continue
		}
		cmd := exec.Command("git", "show", "HEAD:"+path)
		cmd.Dir = source
		data, err := cmd.Output()
		if err != nil {
			t.Fatal("read committed publication input")
		}
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFreshGitCheckoutPublicationInputs(t *testing.T) {
	root := checkoutFixture(t)
	for name, digest := range firmware {
		data, err := os.ReadFile(filepath.Join(root, "firmware", "uhd", name))
		if err != nil {
			t.Fatal("missing committed firmware")
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != digest {
			t.Fatalf("firmware pin differs from Git blob: %s", name)
		}
	}
	if err := validateInputs(root); err != nil {
		t.Fatal(err)
	}
	if err := cleanSeeds(root, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationDiagnosticContextDoesNotLeakValues(t *testing.T) {
	for _, tc := range []struct{ name, path, content, stage string }{
		{"firmware", "firmware/uhd/usrp_b200_fw.hex", "sensitive-fixture-value", "firmware-sha256"},
		{"sidecar", "configs/seeds/sqlite3_init.db-wal", "sensitive-fixture-value", "seed-file-allowlist"},
		{"asterisk", "gsmsystem/asterisk/sip.conf", "secret=sensitive-fixture-value\n", "asterisk-credential-directive"},
		{"config", "configs/app.yaml.example", "token: sensitive-fixture-value\n", "app-config-credential-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := checkoutFixture(t)
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(tc.path)), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			err := validateInputs(root)
			if err == nil || !strings.Contains(err.Error(), "["+tc.stage+"]") {
				t.Fatalf("missing diagnostic stage %s", tc.stage)
			}
			if strings.Contains(err.Error(), "sensitive-fixture-value") {
				t.Fatal("diagnostic exposed a source value")
			}
		})
	}
}

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
