package gsm

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// SMSWindow describes a byte-bounded view of one accepted cell launch.
// 文件身份及字节边界决定会话归属；日志中的时区和时间字符串不参与筛选。
type SMSWindow struct {
	Bytes, MaxBytes int64
	Truncated       bool
	Scope           string
	SessionID       string
	StartedAt       *time.Time
	EndedAt         *time.Time
	State           string
}

type smsCursor struct {
	info   os.FileInfo
	offset int64
	anchor []byte
}

type smsSession struct {
	id      string
	start   smsCursor
	end     *smsCursor
	started time.Time
	ended   *time.Time
	state   string
}

var errSMSBoundary = errors.New("SMS log session boundary is no longer available")

func smsMark(f *os.File) (smsCursor, error) {
	info, err := f.Stat()
	if err != nil {
		return smsCursor{}, err
	}
	if !info.Mode().IsRegular() {
		return smsCursor{}, fmt.Errorf("SMS log is not a regular file")
	}
	c := smsCursor{info: info, offset: info.Size()}
	n := c.offset
	if n > 256 {
		n = 256
	}
	c.anchor = make([]byte, n)
	if n > 0 {
		if _, err := f.ReadAt(c.anchor, c.offset-n); err != nil {
			return smsCursor{}, err
		}
	}
	return c, nil
}

func (c smsCursor) valid(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || !os.SameFile(info, c.info) || info.Size() < c.offset {
		return false
	}
	b := make([]byte, len(c.anchor))
	if len(b) > 0 {
		if _, err := f.ReadAt(b, c.offset-int64(len(b))); err != nil {
			return false
		}
	}
	return bytes.Equal(b, c.anchor)
}

func (m *Manager) clearSMSSession() {
	m.smsMu.Lock()
	m.smsSession = nil
	m.smsMu.Unlock()
}

// beginSMSSession runs immediately before launching native children.
// 创建空日志仅用于建立可验证身份，不清空持久化证据。
func (m *Manager) beginSMSSession() error {
	f, err := os.OpenFile(m.cfg.LogPath(m.cfg.SmqueueLogName), os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	start, err := smsMark(f)
	if err != nil {
		return err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	m.smsMu.Lock()
	m.smsSession = &smsSession{id: hex.EncodeToString(id[:]), start: start, started: time.Now(), state: "starting"}
	m.smsMu.Unlock()
	return nil
}

func (m *Manager) commitSMSSession() {
	m.smsMu.Lock()
	defer m.smsMu.Unlock()
	if m.smsSession != nil && m.smsSession.state == "starting" {
		m.smsSession.state = "running"
	}
}

func (m *Manager) endSMSSession() {
	m.smsMu.Lock()
	defer m.smsMu.Unlock()
	s := m.smsSession
	if s == nil || s.ended != nil {
		return
	}
	now := time.Now()
	s.ended = &now
	if s.state == "boundary_lost" {
		return
	}
	f, err := os.Open(m.cfg.LogPath(m.cfg.SmqueueLogName))
	if err != nil {
		s.state = "boundary_lost"
		return
	}
	defer f.Close()
	end, err := smsMark(f)
	if err != nil {
		s.state = "boundary_lost"
		return
	}
	s.end, s.state = &end, "stopped"
}

// ReadCurrentSMS never adopts preexisting logs after a management restart.
// One normal rename rotation is supported via .1. If the launch boundary has
// been deleted/rewritten, fail closed rather than guessing from timestamps.
// 管理重启无边界返回空；支持一份轮转备份，边界丢失时明确返回空而非历史。
func (m *Manager) ReadCurrentSMS(maxBytes int64) ([]byte, SMSWindow, error) {
	w := SMSWindow{MaxBytes: maxBytes, Scope: "current_start", State: "none"}
	if maxBytes <= 0 {
		return nil, w, fmt.Errorf("SMS history size must be positive")
	}
	// Independent of the long lifecycle mutex: health and SMS requests do not
	// wait for native startup/readiness/shutdown operations.
	m.smsMu.Lock()
	defer m.smsMu.Unlock()
	s := m.smsSession
	if s == nil {
		return nil, w, nil
	}
	started := s.started
	w.SessionID, w.StartedAt, w.State = s.id, &started, s.state
	if s.ended != nil {
		ended := *s.ended
		w.EndedAt = &ended
	}
	if s.state == "boundary_lost" {
		w.Truncated = true
		return nil, w, nil
	}
	data, truncated, err := readSMSSpan(m.cfg.LogPath(m.cfg.SmqueueLogName), s.start, s.end, maxBytes)
	if errors.Is(err, errSMSBoundary) {
		s.state, w.State, w.Truncated = "boundary_lost", "boundary_lost", true
		return nil, w, nil
	}
	w.Bytes, w.Truncated = int64(len(data)), truncated
	return data, w, err
}

type smsSegment struct {
	f          *os.File
	mark       smsCursor
	start, end int64
}

func readSMSSpan(path string, start smsCursor, end *smsCursor, maxBytes int64) ([]byte, bool, error) {
	return readSMSSpanUsing(path, start, end, maxBytes, os.Open)
}

func readSMSSpanUsing(path string, start smsCursor, end *smsCursor, maxBytes int64, open func(string) (*os.File, error)) ([]byte, bool, error) {
	data, truncated, err := readSMSSpanOnce(path, start, end, maxBytes, open)
	if errors.Is(err, errSMSBoundary) {
		// Rotation may occur between the two opens. Resample once before
		// permanently declaring the boundary lost / 跨文件打开遭遇轮转时重采样一次。
		return readSMSSpanOnce(path, start, end, maxBytes, open)
	}
	return data, truncated, err
}

func readSMSSpanOnce(path string, start smsCursor, end *smsCursor, maxBytes int64, open func(string) (*os.File, error)) ([]byte, bool, error) {
	var files []smsSegment
	defer func() {
		for _, segment := range files {
			segment.f.Close()
		}
	}()
	// Open backup first so ordering follows the collector's rename rotation.
	for _, name := range []string{path + ".1", path} {
		f, err := open(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		mark, err := smsMark(f)
		if err != nil {
			f.Close()
			return nil, false, err
		}
		files = append(files, smsSegment{f: f, mark: mark, end: mark.offset})
	}
	first, last := -1, len(files)-1
	for i := range files {
		if start.valid(files[i].f) {
			first = i
			files[i].start = start.offset
		}
	}
	if first < 0 {
		return nil, true, errSMSBoundary
	}
	if end != nil {
		last = -1
		for i := first; i < len(files); i++ {
			if end.valid(files[i].f) {
				last, files[i].end = i, end.offset
			}
		}
		if last < first || files[last].end < files[last].start {
			return nil, true, errSMSBoundary
		}
	}
	// Rotation during opening can expose the same inode under both paths.
	if last > first && os.SameFile(files[first].mark.info, files[last].mark.info) {
		return nil, true, errSMSBoundary
	}
	var total int64
	for i := first; i <= last; i++ {
		total += files[i].end - files[i].start
	}
	skip := int64(0)
	truncated := total > maxBytes
	if truncated {
		skip = total - maxBytes
	}
	dropFirst := len(start.anchor) > 0 && start.anchor[len(start.anchor)-1] != '\n'
	cutChecked := !truncated
	var out bytes.Buffer
	for i := first; i <= last; i++ {
		segment := files[i]
		n := segment.end - segment.start
		if skip >= n {
			skip -= n
			continue
		}
		segment.start += skip
		skip = 0
		if !cutChecked {
			// Keep a full line when the byte budget lands exactly after a
			// newline, including the boundary between rotated files.
			// 上限恰好落在整行边界时保留该行，不无条件丢弃首行。
			previousFile, previousOffset := segment.f, segment.start-1
			if previousOffset < 0 && i > first {
				previousFile, previousOffset = files[i-1].f, files[i-1].end-1
			}
			var previous [1]byte
			if previousOffset >= 0 {
				if _, err := previousFile.ReadAt(previous[:], previousOffset); err != nil {
					return nil, true, errSMSBoundary
				}
				dropFirst = previous[0] != '\n'
			} else {
				dropFirst = false
			}
			cutChecked = true
		}
		if _, err := io.CopyN(&out, io.NewSectionReader(segment.f, segment.start, segment.end-segment.start), segment.end-segment.start); err != nil {
			return nil, true, errSMSBoundary
		}
	}
	for i := first; i <= last; i++ {
		if !files[i].mark.valid(files[i].f) {
			return nil, true, errSMSBoundary
		}
	}
	if !start.valid(files[first].f) || (end != nil && !end.valid(files[last].f)) {
		return nil, true, errSMSBoundary
	}
	b := out.Bytes()
	if dropFirst {
		if p := bytes.IndexByte(b, '\n'); p >= 0 {
			b = b[p+1:]
		} else {
			b = nil
		}
	}
	// In-progress last lines are not messages. The next read includes them
	// once their newline arrives; stopping freezes the incomplete line out.
	if p := bytes.LastIndexByte(b, '\n'); p >= 0 {
		b = b[:p+1]
	} else {
		b = nil
	}
	return b, truncated, nil
}
