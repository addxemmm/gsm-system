// Package config loads stateless tool configuration from env + yaml.
// No database in Go layer: all radio state lives in OpenBTS/Asterisk sqlite
// files + /data/last_start.json. Adapted from lte-system internal/config.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the full server configuration.
type Config struct {
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
	OpenBTSDbPath   string `yaml:"openbts_db_path"`
	TMSITablePath   string `yaml:"tmsi_table_path"`
	AsteriskDbPath  string `yaml:"asterisk_db_path"`
	SmqueueSeedPath string `yaml:"smqueue_seed_path"`

	// Log files.
	SmqueueLogName string `yaml:"smqueue_log_name"`
	OpenBTSLogName string `yaml:"openbts_log_name"`

	// Defaults for /start (explicit GSM radio params, no silent fallback).
	DefaultARFCNs    string `yaml:"default_arfcns"`
	DefaultC0        string `yaml:"default_c0"`
	DefaultBand      string `yaml:"default_band"`
	DefaultMCC       string `yaml:"default_mcc"`
	DefaultMNC       string `yaml:"default_mnc"`
	DefaultLAC       string `yaml:"default_lac"`
	DefaultCI        string `yaml:"default_ci"`
	DefaultShortName string `yaml:"default_short_name"`

	MaxUploadBytes int64 `yaml:"max_upload_bytes"`
}

// Default returns sane defaults matching legacy id=0 preset (test network).
func Default() Config {
	return Config{
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
		SmqueueSeedPath: "/app/configs/smqueue.seed.sql",
		SmqueueLogName:  "smqueue.log",
		OpenBTSLogName:  "openbts.log",
		DefaultARFCNs:   "1",
		DefaultC0:       "540",
		DefaultBand:     "1800",
		DefaultMCC:      "001",
		DefaultMNC:      "01",
		DefaultLAC:      "4420",
		DefaultCI:       "41240",
		DefaultShortName: "test",
		MaxUploadBytes:  8 << 20,
	}
}

// Load reads YAML file if present, then overlays env vars.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return cfg, fmt.Errorf("read config %s: %w", path, err)
			}
		} else if len(b) > 0 {
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config %s: %w", path, err)
			}
		}
	}
	if v := os.Getenv("GSM_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("GSM_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if cfg.ConfDir == "" {
		cfg.ConfDir = filepath.Join(cfg.DataDir, "conf")
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(cfg.DataDir, "log")
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
