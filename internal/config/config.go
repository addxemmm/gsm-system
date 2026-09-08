// Package config loads stateless tool configuration from env + yaml.
// Native subscriber/radio state lives in OpenBTS/Asterisk SQLite
// files; Go owns the API, launch profile and log collection.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the full server configuration.
type Config struct {
	Version    string `yaml:"-"`
	Revision   string `yaml:"-"`
	ListenAddr string `yaml:"listen_addr"`
	DataDir    string `yaml:"data_dir"` // e.g. /data : last_start.json, conf, log live here
	ConfDir    string `yaml:"conf_dir"` // default DataDir/conf
	LogDir     string `yaml:"log_dir"`  // default DataDir/log

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
