package logsink

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Writer appends process output with the same 16 MiB + one backup limit as
// native syslog. Reopening preserves previous evidence rather than truncating.
// 进程输出沿用 16 MiB 加一份备份上限，重新打开时保留已有故障证据。
type Writer struct {
	mu     sync.Mutex
	path   string
	closed bool
}

func NewWriter(path string) (*Writer, error) {
	// Adopt logs created by older unbounded writers using the same tail-retention
	// policy. Copy to a temporary file before replacing; never buffer the old log.
	// 接管旧无界日志时仅保留末尾上限，先写临时文件，避免整份读入或提前截断。
	for _, existing := range []string{path, path + ".1"} {
		if err := capExistingLog(existing); err != nil {
			return nil, err
		}
	}
	if err := appendLog(path, nil, maxLogBytes); err != nil {
		return nil, err
	}
	return &Writer{path: path}, nil
}

func capExistingLog(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("log destination is not a regular file")
	}
	if info.Size() <= maxLogBytes {
		return nil
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := source.Seek(-maxLogBytes, io.SeekEnd); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".bounded-log-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if _, err := io.CopyN(temp, source, maxLogBytes); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func (w *Writer) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	written := 0
	for len(data) > 0 {
		n := len(data)
		if int64(n) > maxLogBytes {
			n = int(maxLogBytes)
		}
		if err := appendLog(w.path, data[:n], maxLogBytes); err != nil {
			return written, err
		}
		written += n
		data = data[n:]
	}
	return written, nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}
