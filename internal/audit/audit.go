package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"userId"`
	Tool       string    `json:"tool"`
	Toolset    string    `json:"toolset"`
	Namespaces []string  `json:"namespaces,omitempty"`
	Resources  []string  `json:"resources,omitempty"`
	Outcome    string    `json:"outcome"`
	Error      string    `json:"error,omitempty"`
}

type Logger struct {
	out io.Writer
	mu  sync.Mutex
}

func NewLogger(out io.Writer) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{out: out}
}

func (l *Logger) Log(event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = l.out.Write(append(data, '\n'))
}
