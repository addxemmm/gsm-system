package gsm

import (
	"io"
	"strings"
	"sync"
)

// processLog detects readiness exclusively in this child session's writes.
// Historical or rotated logs never make a new child ready. Retaining a small
// suffix handles markers split between stdout reads without buffering the log.
// 就绪仅取本次子进程输出；保留短后缀处理跨块标记，不读取历史或整份日志。
type processLog struct {
	mu     sync.Mutex
	output io.WriteCloser
	ready  chan struct{}
	tail   string
	seen   bool
}

func newProcessLog(output io.WriteCloser) *processLog {
	return &processLog{output: output, ready: make(chan struct{})}
}

func (w *processLog) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.output.Write(data)
	if !w.seen {
		const marker = "system ready"
		text := w.tail + string(data[:n])
		if strings.Contains(text, marker) {
			w.seen = true
			close(w.ready)
		} else {
			if len(text) >= len(marker) {
				text = text[len(text)-len(marker)+1:]
			}
			w.tail = text
		}
	}
	return n, err
}

func (w *processLog) Close() error { return w.output.Close() }
