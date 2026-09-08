// Package logsink collects native syslog messages without a Python or syslog daemon.
// 原生 Unix 数据报日志接收器：有界文件、权限限制，不把日志观察冒充投递回执。
package logsink

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

const maxLogBytes int64 = 16 << 20
const maxMessageBytes = 64 << 10

var tagPattern = regexp.MustCompile(`(?i)(?:^|\s)(smqueue|openbts)(?:\[[0-9]+\])?:`)

type Sink struct {
	conn  net.PacketConn
	path  string
	inode os.FileInfo
	done  chan struct{}
	once  sync.Once
}

// Start binds only an unused socket. An existing regular file or live listener
// is never replaced. Empty socketPath explicitly disables collection.
func Start(socketPath, logDir string) (*Sink, error) {
	s := &Sink{}
	if socketPath == "" {
		return s, nil
	}
	if !filepath.IsAbs(socketPath) {
		return nil, fmt.Errorf("syslog socket must be absolute")
	}
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, err
	}
	if old, err := os.Lstat(socketPath); err == nil {
		if old.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("syslog path is not a socket: %s", socketPath)
		}
		probe, dialErr := net.Dial("unixgram", socketPath)
		if dialErr == nil {
			probe.Close()
			return nil, fmt.Errorf("syslog socket already in use: %s", socketPath)
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("check syslog socket: %w", dialErr)
		}
		current, statErr := os.Lstat(socketPath)
		if statErr != nil || !os.SameFile(old, current) {
			return nil, fmt.Errorf("syslog socket changed during startup")
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	conn, err := net.ListenPacket("unixgram", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		conn.Close()
		return nil, err
	}
	inode, err := os.Lstat(socketPath)
	if err != nil {
		conn.Close()
		return nil, err
	}
	s.conn, s.path, s.inode, s.done = conn, socketPath, inode, make(chan struct{})
	go s.receive(logDir)
	return s, nil
}

func (s *Sink) receive(dir string) {
	defer close(s.done)
	buf := make([]byte, maxMessageBytes+1)
	for {
		n, _, err := s.conn.ReadFrom(buf)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("native syslog receiver stopped: %v", err)
			}
			return
		}
		if n > maxMessageBytes {
			log.Printf("native syslog datagram exceeds %d bytes", maxMessageBytes)
			continue
		}
		line := strings.TrimRight(string(buf[:n]), "\x00\r\n")
		if line == "" {
			continue
		}
		if err := appendLog(filepath.Join(dir, logName(line)), []byte(line+"\n"), maxLogBytes); err != nil {
			// Never log the private SMS body when reporting a storage failure.
			log.Printf("native syslog write failed: %v", err)
		}
	}
}

func logName(line string) string {
	match := tagPattern.FindStringSubmatch(line)
	if len(match) > 1 {
		if strings.EqualFold(match[1], "smqueue") {
			return "smqueue.log"
		}
		return "openbts-syslog.log"
	}
	return "system.log"
}

// appendLog retains the current file plus one rotated file. The directory is
// private application storage, never an arbitrary path supplied by HTTP clients.
func appendLog(path string, data []byte, limit int64) error {
	if int64(len(data)) > limit {
		return fmt.Errorf("log record exceeds file limit")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("log destination is not a regular file")
		}
		if info.Size()+int64(len(data)) > limit {
			backup := path + ".1"
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(path, backup); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	return errors.Join(writeErr, closeErr)
}

func (s *Sink) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	var err error
	s.once.Do(func() {
		err = s.conn.Close()
		<-s.done
		if current, statErr := os.Lstat(s.path); statErr == nil && os.SameFile(current, s.inode) {
			err = errors.Join(err, os.Remove(s.path))
		}
	})
	return err
}
