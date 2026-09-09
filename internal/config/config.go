// Package config loads stateless tool configuration from env + yaml.
// Native subscriber/radio state lives in OpenBTS/Asterisk SQLite
// files; Go owns the API, launch profile and log collection.
package config

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Keep IANA zones available in minimal images / 精简镜像内置时区。

	"gopkg.in/yaml.v3"
)

// Config is the full server configuration.
type Config struct {
	Version    string `yaml:"-"`
	Revision   string `yaml:"-"`
	ListenAddr string `yaml:"listen_addr"`
	WebEnabled bool   `yaml:"web_enabled"`
	WebListen  string `yaml:"web_listen"`
	// WebPublicOrigin is the externally visible http(s) origin when TLS is
	// terminated before this process. Empty means derive the origin from r.Host.
	WebPublicOrigin string `yaml:"web_public_origin"`
	Timezone        string `yaml:"timezone"` // IANA name; TZ overrides YAML / TZ 优先于 YAML。
	DataDir         string `yaml:"data_dir"` // e.g. /data : last_start.json, conf, log live here
	ConfDir         string `yaml:"conf_dir"` // default DataDir/conf
	LogDir          string `yaml:"log_dir"`  // default DataDir/log

	// Binaries (absolute paths inside the gsmsystem-dep image).
	OpenBTSBin      string `yaml:"openbts_bin"`
	TransceiverBin  string `yaml:"transceiver_bin"`
	SipAuthServeBin string `yaml:"sipauthserve_bin"`
	SmqueueBin      string `yaml:"smqueue_bin"`
	AsteriskBin     string `yaml:"asterisk_bin"`
	OpenBTSCLIBin   string `yaml:"openbts_cli_bin"`
	Sqlite3Bin      string `yaml:"sqlite3_bin"`
	UHDFindBin      string `yaml:"uhd_find_bin"`
	IptablesBin     string `yaml:"iptables_bin"`

	// OpenBTS/Asterisk sqlite files (container paths).
	OpenBTSDbPath  string `yaml:"openbts_db_path"`
	TMSITablePath  string `yaml:"tmsi_table_path"`
	AsteriskDbPath string `yaml:"asterisk_db_path"`

	// Log files.
	SmqueueLogName  string `yaml:"smqueue_log_name"`
	OpenBTSLogName  string `yaml:"openbts_log_name"`
	SyslogSocket    string `yaml:"syslog_socket"`
	AsteriskCDRPath string `yaml:"asterisk_cdr_path"`

	// Bounded SMS/CDR history reads / 短信与话单读取上限。

	MaxHistoryBytes int64 `yaml:"max_history_bytes"`
}

// Default returns the 2.1 runtime settings; radio parameters are explicit.
func Default() Config {
	return Config{
		Version:         "2.1.0",
		Revision:        "unknown",
		ListenAddr:      ":8082",
		WebEnabled:      true,
		WebListen:       ":18082",
		Timezone:        "Asia/Shanghai",
		DataDir:         "/data",
		OpenBTSBin:      "/OpenBTS/OpenBTS",
		TransceiverBin:  "transceiver",
		SipAuthServeBin: "sipauthserve",
		SmqueueBin:      "smqueue",
		AsteriskBin:     "asterisk",
		OpenBTSCLIBin:   "/OpenBTS/OpenBTSCLI",
		Sqlite3Bin:      "sqlite3",
		UHDFindBin:      "uhd_find_devices",
		IptablesBin:     "iptables",
		OpenBTSDbPath:   "/etc/OpenBTS/OpenBTS.db",
		TMSITablePath:   "/var/run/TMSITable.db",
		AsteriskDbPath:  "/var/lib/asterisk/sqlite3dir/sqlite3.db",
		SmqueueLogName:  "smqueue.log",
		OpenBTSLogName:  "openbts.log",
		SyslogSocket:    "/dev/log",
		AsteriskCDRPath: "/var/log/asterisk/cdr-csv/Master.csv",
		MaxHistoryBytes: 8 << 20,
	}
}

// Load reads YAML file if present, then overlays env vars.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		} else if len(b) > 0 {
			decoder := yaml.NewDecoder(bytes.NewReader(b))
			decoder.KnownFields(true)
			if err := decoder.Decode(&cfg); err != nil {
				return cfg, fmt.Errorf("parse config %s: %w", path, err)
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				return cfg, fmt.Errorf("config must contain exactly one YAML document")
			}
		}
	}
	if v := os.Getenv("GSM_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("GSM_WEB_LISTEN"); v != "" {
		cfg.WebListen = v
	}
	if raw, ok := os.LookupEnv("GSM_WEB_ENABLED"); ok && strings.TrimSpace(raw) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return cfg, fmt.Errorf("GSM_WEB_ENABLED must be true or false: %w", err)
		}
		cfg.WebEnabled = enabled
	}
	if value, ok := os.LookupEnv("GSM_WEB_PUBLIC_ORIGIN"); ok {
		cfg.WebPublicOrigin = value
	}
	if v, ok := os.LookupEnv("TZ"); ok {
		cfg.Timezone = v
	}
	if _, err := cfg.Location(); err != nil {
		return cfg, err
	}
	if cfg.Timezone == "" {
		cfg.Timezone = Default().Timezone
	}
	if v := os.Getenv("GSM_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v, ok := os.LookupEnv("GSM_SYSLOG_SOCKET"); ok {
		cfg.SyslogSocket = v
	}
	if cfg.ConfDir == "" {
		cfg.ConfDir = filepath.Join(cfg.DataDir, "conf")
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(cfg.DataDir, "log")
	}
	if err := validateListenAddress("listen_addr/GSM_LISTEN", cfg.ListenAddr); err != nil {
		return cfg, err
	}
	if cfg.WebEnabled {
		if err := validateListenAddress("web_listen/GSM_WEB_LISTEN", cfg.WebListen); err != nil {
			return cfg, err
		}
	}
	if err := validateWebPublicOrigin(cfg.WebPublicOrigin); err != nil {
		return cfg, err
	}
	if cfg.MaxHistoryBytes < 65536 || cfg.MaxHistoryBytes > 64<<20 {
		return cfg, fmt.Errorf("max_history_bytes must be 65536-67108864")
	}
	for _, name := range []string{cfg.SmqueueLogName, cfg.OpenBTSLogName} {
		if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
			return cfg, fmt.Errorf("log filenames must be plain basenames")
		}
	}
	return cfg, nil
}

func validateWebPublicOrigin(origin string) error {
	if origin == "" {
		return nil
	}
	if strings.TrimSpace(origin) != origin {
		return fmt.Errorf("web_public_origin/GSM_WEB_PUBLIC_ORIGIN must not contain surrounding whitespace")
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("web_public_origin/GSM_WEB_PUBLIC_ORIGIN must be an http(s) origin without credentials, path, query, or fragment")
	}
	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("web_public_origin/GSM_WEB_PUBLIC_ORIGIN contains an empty port")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("web_public_origin/GSM_WEB_PUBLIC_ORIGIN contains an invalid port")
		}
	}
	return nil
}

// validateListenAddress catches configuration mistakes before any receiver,
// HTTP server, or native process is started. Port zero remains valid for
// isolated tests and callers that intentionally request an ephemeral port.
func validateListenAddress(name, address string) error {
	if address == "" || strings.TrimSpace(address) != address {
		return fmt.Errorf("%s must be a host:port listen address", name)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s must be a host:port listen address: %w", name, err)
	}
	if port == "" {
		return fmt.Errorf("%s must include a numeric port", name)
	}
	if strings.TrimSpace(host) != host || strings.ContainsAny(host, "\x00/\\") {
		return fmt.Errorf("%s contains an invalid host", name)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("%s contains an invalid port %q", name, port)
	}
	return nil
}

// EnsureDirs creates DataDir/conf/log.
func (c Config) EnsureDirs() error {
	for _, d := range []string{c.DataDir, c.ConfDir, c.LogDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

// LogPath joins a log filename under LogDir.
func (c Config) LogPath(name string) string { return filepath.Join(c.LogDir, name) }

// Location resolves the project timezone without mutating process-global state.
// 解析项目时区；不改变全局环境，空值使用东八区。
func (c Config) Location() (*time.Location, error) {
	name := c.Timezone
	if name == "" {
		name = "Asia/Shanghai"
	}
	// Reject host-dependent Local and filesystem/POSIX forms: use one named zone
	// for Go and native libc processes / Go 与原生进程统一使用命名时区。
	if name == "Local" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("timezone/TZ must be an IANA timezone name, got %q", name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone/TZ %q: %w", name, err)
	}
	return loc, nil
}
