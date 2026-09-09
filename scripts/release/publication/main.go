// Publication input validation and clean SQLite seeds / 发布输入校验与干净种子。
// Uses only Go stdlib and sqlite3 CLI; never prints source values.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"usrp_b200_bl.img":            "365a70342fa79ab766c4df7f78713055f0eebcd1c68588b806a19dbbf97d0b48",
	"usrp_b200_fw.hex":            "cc8e4bc968d91d1a67c63871ce5b2c1613c49676fe363bf2f6e336059899ae5c",
	"usrp_b210_fpga.blacksdr.bin": "8e2acce1f987d8452d845705c9c27879171724a99b87c640d0e1473e256fe69a",
}
var tables = map[string][]string{
	"sqlite3_init.db":   {"DIALDATA_TABLE", "RRLP", "SIP_BUDDIES", "rates"},
	"TMSITable_init.db": {"ATTR_TABLE", "TMSI_TABLE"},
}

func fail() error {
	return errors.New("publication input validation failed / 发布输入校验失败（不输出源数据）")
}

func query(path, sql string, readonly bool) ([]byte, error) {
	if readonly {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fail()
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
		return nil, fail()
	}
	return output, nil
}

func cleanSeeds(root, output string) error {
	for name, expected := range tables {
		path := filepath.Join(root, "configs", "seeds", name)
		check, err := query(path, "PRAGMA quick_check;", true)
		if err != nil || !strings.Contains(string(check), `"quick_check":"ok"`) {
			return fail()
		}
		rows, err := query(path, "SELECT type,name,sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY rowid;", true)
		if err != nil {
			return err
		}
		var schema []struct{ Type, Name, SQL string }
		if json.Unmarshal(rows, &schema) != nil {
			return fail()
		}
		var actual []string
		var statements []string
		for _, row := range schema {
			if row.Type != "table" {
				return fail()
			}
			actual = append(actual, row.Name)
			statements = append(statements, row.SQL+";")
		}
		sort.Strings(actual)
		if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
			return fail()
		}
		for _, table := range expected {
			if table == "ATTR_TABLE" {
				data, err := query(path, "SELECT ATTR_NAME,ATTR_VALUE FROM ATTR_TABLE;", true)
				var attrs []struct {
					Name  string `json:"ATTR_NAME"`
					Value string `json:"ATTR_VALUE"`
				}
				if err != nil || json.Unmarshal(data, &attrs) != nil || len(attrs) != 1 || attrs[0].Name != "VERSION" || attrs[0].Value != "7" {
					return fail()
				}
				statements = append(statements, "INSERT INTO ATTR_TABLE VALUES ('VERSION','7');")
			} else {
				data, err := query(path, `SELECT COUNT(*) AS n FROM "`+table+`";`, true)
				var counts []struct{ N int }
				if err != nil || json.Unmarshal(data, &counts) != nil || len(counts) != 1 || counts[0].N != 0 {
					return fail()
				}
			}
		}
		if output != "" {
			if os.MkdirAll(output, 0755) != nil {
				return fail()
			}
			destination := filepath.Join(output, name)
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				return fail()
			}
			// Rebuild schema, never copy deleted records or historical free pages.
			// 重建 schema，不复制历史页和已删除记录。
			if _, err := query(destination, strings.Join(statements, "\n"), false); err != nil {
				return err
			}
			data, err := query(destination, "PRAGMA freelist_count;", true)
			if err != nil || !strings.Contains(string(data), `"freelist_count":0`) {
				return fail()
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
			return fail()
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != digest {
			return fail()
		}
	}
	if !namesEqual(filepath.Join(root, "configs"), []string{"app.yaml.example", "seeds"}) ||
		!namesEqual(filepath.Join(root, "configs", "seeds"), []string{"OpenBTSDo", "sqlite3_init.db", "TMSITable_init.db"}) {
		return fail()
	}
	entries, err := os.ReadDir(filepath.Join(root, "gsmsystem", "asterisk"))
	if err != nil {
		return fail()
	}
	credential := regexp.MustCompile(`(?i)(?:secret|password|passwd|token)\s*=\s*[^;\s]|register\s*=>`)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			return fail()
		}
		data, err := os.ReadFile(filepath.Join(root, "gsmsystem", "asterisk", entry.Name()))
		if err != nil {
			return fail()
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
				continue
			}
			if credential.MatchString(line) {
				return fail()
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "configs", "app.yaml.example"))
	if err != nil || regexp.MustCompile(`(?im)^\s*(?:token|password|secret|ki)\s*:`).Match(data) {
		return fail()
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
