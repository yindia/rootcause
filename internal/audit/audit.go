package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"userId"`
	TraceID    string    `json:"traceId,omitempty"`
	ParentTool string    `json:"parentTool,omitempty"`
	CallChain  []string  `json:"callChain,omitempty"`
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

var jsonMarshal = json.Marshal

func NewLogger(out io.Writer) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{out: out}
}

func (l *Logger) Log(event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := jsonMarshal(event)
	if err != nil {
		// Don't go silent: a missing audit line is worse than an ugly one.
		// Emit a minimal JSON record so the operator sees that audit is
		// broken (and which tool was running at the time).
		fallback := fmt.Sprintf(
			`{"timestamp":%q,"audit_error":%q,"tool":%q,"toolset":%q,"outcome":"audit_error"}`+"\n",
			event.Timestamp.UTC().Format(time.RFC3339Nano),
			err.Error(),
			event.Tool,
			event.Toolset,
		)
		_, _ = l.out.Write([]byte(fallback))
		return
	}
	_, _ = l.out.Write(append(data, '\n'))
}
