// Publication input validation and clean SQLite seeds / 发布输入校验与干净种子。
// Uses only Go stdlib and sqlite3 CLI; never prints source values.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var firmware = map[string]string{
	"usrp_b200_bl.img": "365a70342fa79ab766c4df7f78713055f0eebcd1c68588b806a19dbbf97d0b48",
	// Pin the Git blob (LF), not an old Windows checkout's CRLF conversion.
	// 固定 Git 原始 blob（LF），不是旧 Windows 工作树的 CRLF 转换结果。
	"usrp_b200_fw.hex":            "8a200144aa4f2a5b82db4caf989c8b88a29fc3634108794e5a079d9fa1c4e877",
	"usrp_b210_fpga.blacksdr.bin": "8e2acce1f987d8452d845705c9c27879171724a99b87c640d0e1473e256fe69a",
}
var tables = map[string][]string{
	"sqlite3_init.db":   {"DIALDATA_TABLE", "RRLP", "SIP_BUDDIES", "rates"},
	"TMSITable_init.db": {"ATTR_TABLE", "TMSI_TABLE"},
}

func fail(stage, file string) error {
	// Only caller-supplied static stages/known input names, never values, SQL or
	// subprocess output. This makes CI failures actionable without leaking data.
	// 仅打印固定阶段及已知文件名，不打印值、SQL 或子进程输出，便于定位 CI 失败。
	return fmt.Errorf("publication input validation failed [%s] %s / 发布输入校验失败（不输出源数据）", stage, file)
}

func query(path, sql string, readonly bool) ([]byte, error) {
	if readonly {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fail("sqlite-path", "seed database")
		}
		p := filepath.ToSlash(absolute)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		path = (&url.URL{Scheme: "file", Path: p, RawQuery: "mode=ro&immutable=1"}).String()
	}
	cmd := exec.Command("sqlite3", "-batch", "-bail", "-json", path)
	cmd.Stdin = strings.NewReader(sql)
	output, err := cmd.Output()
	if err != nil {
		return nil, fail("sqlite-exec", "seed database; sqlite3 command required")
	}
	return output, nil
}

func cleanSeeds(root, output string) error {
	for name, expected := range tables {
		path := filepath.Join(root, "configs", "seeds", name)
		check, err := query(path, "PRAGMA quick_check;", true)
		if err != nil || !strings.Contains(string(check), `"quick_check":"ok"`) {
			return fail("seed-integrity", name)
		}
		rows, err := query(path, "SELECT type,name,sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY rowid;", true)
		if err != nil {
			return fail("seed-schema-query", name)
		}
		var schema []struct{ Type, Name, SQL string }
		if json.Unmarshal(rows, &schema) != nil {
			return fail("seed-schema-json", name)
		}
		var actual []string
		var statements []string
		for _, row := range schema {
			if row.Type != "table" {
				return fail("seed-schema-object", name)
			}
			actual = append(actual, row.Name)
			statements = append(statements, row.SQL+";")
		}
		sort.Strings(actual)
		if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
			return fail("seed-table-allowlist", name)
		}
		for _, table := range expected {
			if table == "ATTR_TABLE" {
				data, err := query(path, "SELECT ATTR_NAME,ATTR_VALUE FROM ATTR_TABLE;", true)
				var attrs []struct {
					Name  string `json:"ATTR_NAME"`
					Value string `json:"ATTR_VALUE"`
				}
				if err != nil || json.Unmarshal(data, &attrs) != nil || len(attrs) != 1 || attrs[0].Name != "VERSION" || attrs[0].Value != "7" {
					return fail("seed-version-metadata", name)
				}
				statements = append(statements, "INSERT INTO ATTR_TABLE VALUES ('VERSION','7');")
			} else {
				data, err := query(path, `SELECT COUNT(*) AS n FROM "`+table+`";`, true)
				var counts []struct{ N int }
				if err != nil || json.Unmarshal(data, &counts) != nil || len(counts) != 1 || counts[0].N != 0 {
					return fail("seed-business-empty", name)
				}
			}
		}
		if output != "" {
			if os.MkdirAll(output, 0755) != nil {
				return fail("seed-output-directory", name)
			}
			destination := filepath.Join(output, name)
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				return fail("seed-output-exists", name)
			}
			// Rebuild schema, never copy deleted records or historical free pages.
			// 重建 schema，不复制历史页和已删除记录。
			if _, err := query(destination, strings.Join(statements, "\n"), false); err != nil {
				return fail("seed-rebuild", name)
			}
			data, err := query(destination, "PRAGMA freelist_count;", true)
			if err != nil || !strings.Contains(string(data), `"freelist_count":0`) {
				return fail("seed-rebuilt-freelist", name)
			}
		}
	}
	return nil
}

func namesEqual(path string, expected []string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	var actual []string
	for _, entry := range entries {
		actual = append(actual, entry.Name())
	}
	sort.Strings(expected)
	return strings.Join(actual, "\n") == strings.Join(expected, "\n")
}

func validateInputs(root string) error {
	for name, digest := range firmware {
		data, err := os.ReadFile(filepath.Join(root, "firmware", "uhd", name))
		if err != nil {
			return fail("firmware-read", name)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != digest {
			return fail("firmware-sha256", name)
		}
	}
	if !namesEqual(filepath.Join(root, "configs"), []string{"app.yaml.example", "seeds"}) {
		return fail("config-file-allowlist", "configs/")
	}
	if !namesEqual(filepath.Join(root, "configs", "seeds"), []string{"OpenBTSDo", "sqlite3_init.db", "TMSITable_init.db"}) {
		return fail("seed-file-allowlist", "configs/seeds/; unexpected files or SQLite sidecars")
	}
	entries, err := os.ReadDir(filepath.Join(root, "gsmsystem", "asterisk"))
	if err != nil {
		return fail("asterisk-directory", "gsmsystem/asterisk/")
	}
	credential := regexp.MustCompile(`(?i)(?:secret|password|passwd|token)\s*=\s*[^;\s]|register\s*=>`)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			return fail("asterisk-file-allowlist", "gsmsystem/asterisk/")
		}
		data, err := os.ReadFile(filepath.Join(root, "gsmsystem", "asterisk", entry.Name()))
		if err != nil {
			return fail("asterisk-config-read", "gsmsystem/asterisk/")
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
				continue
			}
			if credential.MatchString(line) {
				return fail("asterisk-credential-directive", "gsmsystem/asterisk/")
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "configs", "app.yaml.example"))
	if err != nil {
		return fail("app-config-read", "configs/app.yaml.example")
	}
	if regexp.MustCompile(`(?im)^\s*(?:token|password|secret|ki)\s*:`).Match(data) {
		return fail("app-config-credential-key", "configs/app.yaml.example")
	}
	return nil
}

func main() {
	seedsOnly := flag.Bool("seeds-only", false, "validate seeds only / 仅检查种子")
	output := flag.String("output-seeds", "", "new clean seed directory / 新干净种子目录")
	flag.Parse()
	if !*seedsOnly {
		if err := validateInputs("."); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := cleanSeeds(".", *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("PASS publication inputs / 发布输入校验通过")
}
