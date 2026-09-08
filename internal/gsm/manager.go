// Manager owns the OpenBTS cell lifecycle (stateless Go side).
// Processes: OpenBTS, transceiver, sipauthserve, smqueue, asterisk.
package gsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

var (
	// ErrCellRunning is returned when a config/start operation races a live cell.
	ErrCellRunning   = errors.New("cell is running")
	ErrInvalidConfig = errors.New("invalid radio configuration")
	// ErrDatabaseNotFound classifies a missing configured sqlite database.
	ErrDatabaseNotFound = errors.New("database file not found")
)

type managedProcess struct {
	name string
	cmd  *exec.Cmd
	done chan struct{}
}

func (p *managedProcess) alive() bool {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Manager owns child handles + saved profile.
type Manager struct {
	cfg       config.Config
	mu        sync.Mutex
	profileMu sync.Mutex

	openbts       *managedProcess
	ownedChildren []*managedProcess
	startedAt     time.Time
	lastStart     StartParams
}

// New creates a Manager.
func New(cfg config.Config) *Manager { return &Manager{cfg: cfg} }

// Status is the machine-readable state for /api/v1/cell and /health.
type Status struct {
	State         string     `json:"state"`
	Ready         bool       `json:"ready"`
	SMSReady      bool       `json:"sms_ready"`
	VoiceReady    bool       `json:"voice_ready"`
	Transitioning bool       `json:"transitioning"`
	Running       bool       `json:"running"`
	OpenBTS       bool       `json:"openbts"`
	Transc        bool       `json:"transceiver"`
	SipAuth       bool       `json:"sipauthserve"`
	Smqueue       bool       `json:"smqueue"`
	Asterisk      bool       `json:"asterisk"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	Band          string     `json:"band,omitempty"`
	ShortName     string     `json:"short_name,omitempty"`
}

// IsRunning reports live state (system procs OR managed child).
func (m *Manager) IsRunning() Status {
	st := Status{
		OpenBTS:  sysop.Running("OpenBTS"),
		Transc:   sysop.Running("transceiver"),
		SipAuth:  sysop.Running("sipauthserve"),
		Smqueue:  sysop.Running("smqueue"),
		Asterisk: sysop.Running("asterisk"),
	}
	// Long start/stop operations must not block health probes for their full
	// timeout. Process observations remain available while metadata is locked.
	if !m.mu.TryLock() {
		st.Transitioning = true
		st.classify()
		return st
	}
	defer m.mu.Unlock()
	if m.openbts.alive() {
		st.OpenBTS = true
	}
	for _, child := range m.ownedChildren {
		if child.alive() {
			switch child.name {
			case "sipauthserve":
				st.SipAuth = true
			case "smqueue":
				st.Smqueue = true
			case "asterisk":
				st.Asterisk = true
			}
		}
	}
	st.classify()
	if st.Running && !m.startedAt.IsZero() {
		t := m.startedAt
		st.StartedAt = &t
		st.Band = m.lastStart.Band
		st.ShortName = m.lastStart.ShortName
	}
	return st
}

// Ready means process availability, not a successful handset/RF test.
func (st *Status) classify() {
	st.Running = st.OpenBTS || st.Transc
	st.SMSReady = st.OpenBTS && st.Transc && st.SipAuth && st.Smqueue
	st.VoiceReady = st.OpenBTS && st.Transc && st.SipAuth && st.Asterisk
	st.Ready = st.SMSReady && st.VoiceReady && !st.Transitioning
	st.State = "degraded"
	if !st.Running && !st.SipAuth && !st.Smqueue && !st.Asterisk {
		st.State = "stopped"
	}
	if st.Ready {
		st.State = "running"
	}
	if st.Transitioning {
		st.State = "transitioning"
	}
}

// WithStoppedCell serializes a configuration side effect against Start/Stop.
func (m *Manager) WithStoppedCell(operation func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cellRunningLocked() {
		return ErrCellRunning
	}
	return operation()
}

// CheckUSRP reports B210 presence.
func (m *Manager) CheckUSRP() bool {
	return m.DetectUSRP().UHD_B210
}

// DetectUSRP returns the configured detector result for health endpoints.
func (m *Manager) DetectUSRP() sdr.Detection { return sdr.DetectWith(m.cfg.UHDFindBin) }

// Start validates, refuses when running, checks USRP, applies DB config,
// launches sipauthserve -> smqueue -> asterisk -> OpenBTS (direct exec,
// no systemctl: containers have no systemd), clears tmsis.
func (m *Manager) Start(ctx context.Context, p StartParams) (err error) {
	if err := p.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.cellRunningLocked() {
		return ErrCellRunning
	}
	if err := checkIface(p.Network); err != nil {
		return err
	}
	if err := m.cfg.EnsureDirs(); err != nil {
		return err
	}
	if !m.DetectUSRP().UHD_B210 {
		return fmt.Errorf("device is not connected, please connect usrp device.")
	}
	// Best-effort stale pid cleanup (legacy `rm /var/run/OpenBTS.pid`).
	_ = os.Remove("/var/run/OpenBTS.pid")

	// Apply radio config to OpenBTS.db before launch (preset-equivalent).
	if err := m.applyConfigLocked(p); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Only children launched by this start attempt are rolled back. Pre-existing
	// system services are deliberately left alone.
	var owned []*managedProcess
	committed := false
	defer func() {
		if !committed {
			stopManagedReverse(owned, 2*time.Second)
		}
	}()

	// Launch order mirrors v1.3/run.sh (direct binaries, not systemctl).
	if !sysop.Running("sipauthserve") {
		child, startErr := startDetached("sipauthserve", m.cfg.SipAuthServeBin)
		if startErr != nil {
			return fmt.Errorf("start sipauthserve: %w", startErr)
		}
		owned = append(owned, child)
	}
	if !sysop.Running("smqueue") {
		child, startErr := startDetached("smqueue", m.cfg.SmqueueBin)
		if startErr != nil {
			return fmt.Errorf("start smqueue: %w", startErr)
		}
		owned = append(owned, child)
	}
	if !sysop.Running("asterisk") {
		child, startErr := startDetached("asterisk", m.cfg.AsteriskBin, "-f", "-g")
		if startErr != nil {
			return fmt.Errorf("start asterisk: %w", startErr)
		}
		owned = append(owned, child)
	}
	if err := waitContext(ctx, 2*time.Second); err != nil {
		return err
	}
	for _, child := range owned {
		if !child.alive() {
			return fmt.Errorf("%s exited during startup", child.name)
		}
	}

	// Start OpenBTS (foreground binary, detached).
	// CWD must be /OpenBTS: OpenBTS execs ./transceiver by relative path
	// (legacy Flask ran with supervisord directory=/OpenBTS for the same reason).
	cmd := exec.Command(m.cfg.OpenBTSBin)
	configureProcessGroup(cmd)
	if filepath.IsAbs(m.cfg.OpenBTSBin) {
		cmd.Dir = filepath.Dir(m.cfg.OpenBTSBin)
	}
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
	openbts := watchProcess("OpenBTS", cmd, logF)
	owned = append(owned, openbts)
	// Wait for "system ready" (max ~20s), like run.sh timer-loopback wait.
	if err := waitForReady(ctx, m.cfg.LogPath(m.cfg.OpenBTSLogName), openbts.done, 20*time.Second); err != nil {
		return fmt.Errorf("OpenBTS did not become ready, see %s: %w", m.cfg.LogPath(m.cfg.OpenBTSLogName), err)
	}
	// Legacy run.py: clear tmsis after successful start. It is best effort,
	// but bounded and request cancellation still rolls back this start attempt.
	cliCtx, cliCancel := context.WithTimeout(ctx, 2*time.Second)
	runBoundedBestEffort(cliCtx, m.cfg.OpenBTSCLIBin, "-c", "tmsis", "clear")
	cliCancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, child := range owned {
		if !child.alive() {
			return fmt.Errorf("%s exited before startup completed", child.name)
		}
	}
	if err := m.SaveProfile(p); err != nil {
		return fmt.Errorf("save start profile: %w", err)
	}

	m.startedAt = time.Now()
	m.lastStart = p
	m.openbts = openbts
	m.ownedChildren = append(m.ownedChildren[:0], owned[:len(owned)-1]...)
	committed = true
	return nil
}

// Stop kills smqueue -> asterisk -> sipauthserve -> OpenBTS -> transceiver,
// removes pid file, clears tmsis best-effort. Returns true when stopped.
func (m *Manager) Stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	was, stopped := m.stopLocked()
	return was && stopped
}

// stopLocked must be called with m.mu held. All waits and CLI calls are bounded.
func (m *Manager) stopLocked() (was, stopped bool) {
	mainWasRunning := m.cellRunningLocked()
	was = m.anyServiceRunningLocked()
	// The CLI needs OpenBTS alive, so clear state before terminating the main
	// process. A wedged CLI must not wedge shutdown.
	if mainWasRunning {
		cliCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		runBoundedBestEffort(cliCtx, m.cfg.OpenBTSCLIBin, "-c", "tmsis", "clear")
		cancel()
	}

	stopManagedReverse(m.ownedChildren, 2*time.Second)
	m.ownedChildren = nil
	stopManaged(m.openbts, 2*time.Second)
	m.openbts = nil

	sysop.KillAll("smqueue", 2*time.Second)
	sysop.KillAll("asterisk", 2*time.Second)
	sysop.KillAll("sipauthserve", 2*time.Second)
	_ = os.Remove("/var/run/OpenBTS.pid")
	sysop.KillAll("OpenBTS", 3*time.Second)
	sysop.KillAll("transceiver", 3*time.Second)
	stopped = !m.anyServiceRunningLocked()
	if stopped {
		m.startedAt = time.Time{}
	}
	return was, stopped
}

// ProfilePath is /data/last_start.json (survives recreates via volume).
func (m *Manager) ProfilePath() string {
	return filepath.Join(m.cfg.DataDir, "last_start.json")
}

// SaveProfile persists resolved start params (atomic tmp+rename).
func (m *Manager) SaveProfile(p StartParams) error {
	m.profileMu.Lock()
	defer m.profileMu.Unlock()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.cfg.DataDir, ".last-start-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), m.ProfilePath()); err != nil {
		return err
	}
	return syncProfileDirectory(m.cfg.DataDir)
}

// LoadProfile reads the persisted launch config.
func (m *Manager) LoadProfile() (StartParams, bool) {
	m.profileMu.Lock()
	defer m.profileMu.Unlock()
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

func startDetached(name, bin string, args ...string) (*managedProcess, error) {
	cmd := exec.Command(bin, args...)
	configureProcessGroup(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return watchProcess(name, cmd, nil), nil
}

func watchProcess(name string, cmd *exec.Cmd, closer io.Closer) *managedProcess {
	p := &managedProcess{name: name, cmd: cmd, done: make(chan struct{})}
	// This is the unique Wait owner. Closing the log here also covers a normal
	// post-start process exit without leaking the parent file descriptor.
	go func() {
		_ = cmd.Wait()
		if closer != nil {
			_ = closer.Close()
		}
		close(p.done)
	}()
	return p
}

func waitForReady(ctx context.Context, logPath string, done <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf("process exited")
		default:
		}
		b, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(b), "system ready") {
			select {
			case <-done:
				return fmt.Errorf("process exited")
			default:
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf("process exited")
		case <-timer.C:
			return fmt.Errorf("readiness timeout")
		case <-ticker.C:
		}
	}
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stopManaged(p *managedProcess, timeout time.Duration) {
	if p == nil {
		return
	}
	// TERM gives native services time to flush CDRs/SQLite. KILL still covers
	// descendants after the parent exits, using a fresh bounded wait.
	groupErr := terminateProcessGroup(p.cmd)
	if groupErr != nil && p.alive() {
		// Compatibility fallback for a caller-supplied Cmd that was not started
		// through startDetached/configureProcessGroup.
		_ = p.cmd.Process.Kill()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
	case <-timer.C:
	}
	_ = killProcessGroup(p.cmd)
	select {
	case <-p.done:
	case <-time.After(time.Second):
	}
}

func runBoundedBestEffort(ctx context.Context, bin string, args ...string) {
	if ctx.Err() != nil {
		return
	}
	cmd := exec.Command(bin, args...)
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return
	}
	p := watchProcess(filepath.Base(bin), cmd, nil)
	select {
	case <-p.done:
		return
	case <-ctx.Done():
		_ = killProcessGroup(cmd)
		select {
		case <-p.done:
		case <-time.After(time.Second):
		}
	}
}

func stopManagedReverse(children []*managedProcess, timeout time.Duration) {
	for i := len(children) - 1; i >= 0; i-- {
		stopManaged(children[i], timeout)
	}
}

func (m *Manager) cellRunningLocked() bool {
	return m.openbts.alive() || sysop.Running("OpenBTS") || sysop.Running("transceiver")
}

func (m *Manager) anyServiceRunningLocked() bool {
	if m.openbts.alive() {
		return true
	}
	for _, child := range m.ownedChildren {
		if child.alive() {
			return true
		}
	}
	_, running := sysop.AnyRunning(CellProcs...)
	return running
}
