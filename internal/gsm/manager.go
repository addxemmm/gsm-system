// Manager owns the OpenBTS cell lifecycle (stateless Go side).
// Processes: OpenBTS, transceiver, sipauthserve, smqueue, asterisk.
package gsm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/sdr"
	"github.com/addxemmm/gsm-system/internal/sysop"
)

// CellProcs in start-check order (legacy run.py checks 4, stop checks 5).
var CellProcs = []string{"OpenBTS", "sipauthserve", "smqueue", "asterisk", "transceiver"}

// Manager owns child handles + saved profile.
type Manager struct {
	cfg config.Config
	mu  sync.Mutex

	openbtsCmd *exec.Cmd
	startedAt  time.Time
	lastStart  StartParams
	lastNet    string
}

// New creates a Manager.
func New(cfg config.Config) *Manager { return &Manager{cfg: cfg} }

// Status is the machine-readable state for /api/v1/cell and /health.
type Status struct {
	Running   bool       `json:"running"`
	OpenBTS   bool       `json:"openbts"`
	Transc    bool       `json:"transceiver"`
	SipAuth   bool       `json:"sipauthserve"`
	Smqueue   bool       `json:"smqueue"`
	Asterisk  bool       `json:"asterisk"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	Band      string     `json:"band,omitempty"`
	ShortName string     `json:"short_name,omitempty"`
}

// IsRunning reports live state (system procs OR managed child).
func (m *Manager) IsRunning() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{
		OpenBTS:  sysop.Running("OpenBTS"),
		Transc:   sysop.Running("transceiver"),
		SipAuth:  sysop.Running("sipauthserve"),
		Smqueue:  sysop.Running("smqueue"),
		Asterisk: sysop.Running("asterisk"),
	}
	if m.openbtsCmd != nil && m.openbtsCmd.Process != nil {
		st.OpenBTS = st.OpenBTS || true
	}
	st.Running = st.OpenBTS || st.Transc
	if st.Running && !m.startedAt.IsZero() {
		t := m.startedAt
		st.StartedAt = &t
		st.Band = m.lastStart.Band
		st.ShortName = m.lastStart.ShortName
	}
	return st
}

// CheckUSRP reports B210 presence.
func (m *Manager) CheckUSRP() bool {
	return sdr.Detect().UHD_B210
}

// Start validates, refuses when running, checks USRP, applies DB config,
// launches sipauthserve -> smqueue -> asterisk -> OpenBTS (direct exec,
// no systemctl: containers have no systemd), clears tmsis.
func (m *Manager) Start(ctx context.Context, p StartParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if sysop.Running("OpenBTS") {
		return fmt.Errorf("is running")
	}
	if err := checkIface(p.Network); err != nil {
		return err
	}
	if err := m.cfg.EnsureDirs(); err != nil {
		return err
	}
	if !sdr.Detect().UHD_B210 {
		return fmt.Errorf("device is not connected, please connect usrp device.")
	}
	// Best-effort stale pid cleanup (legacy `rm /var/run/OpenBTS.pid`).
	_ = os.Remove("/var/run/OpenBTS.pid")

	// Apply radio config to OpenBTS.db before launch (preset-equivalent).
	if err := m.applyConfigLocked(p); err != nil {
		return err
	}
	// Launch order mirrors v1.3/run.sh (direct binaries, not systemctl).
	if !sysop.Running("sipauthserve") {
		_ = startDetached(m.cfg.SipAuthServeBin)
	}
	if !sysop.Running("smqueue") {
		_ = startDetached(m.cfg.SmqueueBin)
	}
	if !sysop.Running("asterisk") {
		_ = startDetached(m.cfg.AsteriskBin, "-f", "-g")
	}
	time.Sleep(2 * time.Second)

	// Reset smqueue seed (stateless: clean queue every start, documented).
	_ = m.resetSmqueueLocked()

	// Start OpenBTS (foreground binary, detached).
	// CWD must be /OpenBTS: OpenBTS execs ./transceiver by relative path
	// (legacy Flask ran with supervisord directory=/OpenBTS for the same reason).
	cmd := exec.CommandContext(context.Background(), m.cfg.OpenBTSBin)
	cmd.Dir = "/OpenBTS"
	logF, err := os.Create(m.cfg.LogPath(m.cfg.OpenBTSLogName))
	if err != nil {
		return err
	}
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return fmt.Errorf("start OpenBTS: %w", err)
	}
	m.openbtsCmd = cmd
	// Wait for "system ready" (max ~20s), like run.sh timer-loopback wait.
	if !waitForReady(m.cfg.LogPath(m.cfg.OpenBTSLogName), 20*time.Second) {
		_ = cmd.Process.Kill()
		reap(cmd)
		m.openbtsCmd = nil
		_ = logF.Close()
		return fmt.Errorf("OpenBTS did not become ready, see %s", m.cfg.LogPath(m.cfg.OpenBTSLogName))
	}
	// Legacy run.py: clear tmsis after successful start.
	_ = exec.Command(m.cfg.OpenBTSCLIBin, "-c", "tmsis", "clear").Run()

	m.startedAt = time.Now()
	m.lastStart = p
	m.lastNet = p.Network
	_ = ctx
	_ = m.SaveProfile(p)
	return nil
}

// Stop kills smqueue -> asterisk -> sipauthserve -> OpenBTS -> transceiver,
// removes pid file, clears tmsis best-effort. Returns true when stopped.
func (m *Manager) Stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	was := sysop.Running("OpenBTS") || sysop.Running("transceiver") ||
		sysop.Running("sipauthserve") || sysop.Running("smqueue") || sysop.Running("asterisk")
	if m.openbtsCmd != nil && m.openbtsCmd.Process != nil {
		_ = m.openbtsCmd.Process.Kill()
		reap(m.openbtsCmd)
		m.openbtsCmd = nil
	}
	// Best-effort tmsis clear before kill (legacy stop order).
	_ = exec.Command(m.cfg.OpenBTSCLIBin, "-c", "tmsis", "clear").Run()
	sysop.KillAll("smqueue", 2*time.Second)
	sysop.KillAll("asterisk", 2*time.Second)
	sysop.KillAll("sipauthserve", 2*time.Second)
	_ = os.Remove("/var/run/OpenBTS.pid")
	sysop.KillAll("OpenBTS", 3*time.Second)
	sysop.KillAll("transceiver", 3*time.Second)
	time.Sleep(time.Second)
	still := sysop.Running("OpenBTS") || sysop.Running("transceiver") ||
		sysop.Running("sipauthserve") || sysop.Running("smqueue") || sysop.Running("asterisk")
	if was && !still {
		m.lastNet = ""
		m.startedAt = time.Time{}
	}
	return was && !still
}

// ProfilePath is /data/last_start.json (survives recreates via volume).
func (m *Manager) ProfilePath() string {
	return filepath.Join(m.cfg.DataDir, "last_start.json")
}

// SaveProfile persists resolved start params (atomic tmp+rename).
func (m *Manager) SaveProfile(p StartParams) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.ProfilePath() + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.ProfilePath())
}

// LoadProfile reads the persisted launch config.
func (m *Manager) LoadProfile() (StartParams, bool) {
	b, err := os.ReadFile(m.ProfilePath())
	if err != nil {
		return StartParams{}, false
	}
	var v StartParams
	if json.Unmarshal(b, &v) != nil {
		return StartParams{}, false
	}
	if strings.TrimSpace(v.Band) == "" || strings.TrimSpace(v.MCC) == "" ||
		strings.TrimSpace(v.MNC) == "" || strings.TrimSpace(v.Network) == "" {
		return StartParams{}, false
	}
	return v, true
}

// OverlayProfile fills every empty field of p from the saved profile.
func (m *Manager) OverlayProfile(p *StartParams) bool {
	saved, ok := m.LoadProfile()
	if !ok {
		return false
	}
	if p.ARFCNs == "" {
		p.ARFCNs = saved.ARFCNs
	}
	if p.C0 == "" {
		p.C0 = saved.C0
	}
	if p.Band == "" {
		p.Band = saved.Band
	}
	if p.MCC == "" {
		p.MCC = saved.MCC
	}
	if p.MNC == "" {
		p.MNC = saved.MNC
	}
	if p.LAC == "" {
		p.LAC = saved.LAC
	}
	if p.CI == "" {
		p.CI = saved.CI
	}
	if p.ShortName == "" {
		p.ShortName = saved.ShortName
	}
	if p.Network == "" {
		p.Network = saved.Network
	}
	return true
}

func checkIface(name string) error {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil || len(entries) == 0 {
		return nil // non-Linux dev machine: skip (unit tests)
	}
	for _, e := range entries {
		if e.Name() == name {
			return nil
		}
	}
	return fmt.Errorf("unknown network interface %q", name)
}

func startDetached(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap on exit: otherwise every start leaves <defunct> zombies
	// parented to PID 1 (observed live on vm-sdr 2026-09-06).
	go func() { _ = cmd.Wait() }()
	return nil
}

func waitForReady(logPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(b), "system ready") {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func reap(cmd *exec.Cmd) {
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
