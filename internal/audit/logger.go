package audit

import (
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
)

type AuditLogger interface {
	Log(AuditEvent)
}

type Logger struct {
	mu          sync.RWMutex
	closed      bool
	events      chan AuditEvent
	w           io.Writer
	dropped     atomic.Int64
	writeErrors atomic.Int64
	done        chan struct{}
	wg          sync.WaitGroup
}

func NewLogger(w io.Writer, bufferSize int) *Logger {
	l := &Logger{
		w:      w,
		done:   make(chan struct{}),
		events: make(chan AuditEvent, bufferSize),
	}
	l.wg.Add(1)
	go l.run()
	return l
}

func (l *Logger) run() {
	defer l.wg.Done()
	enc := json.NewEncoder(l.w)
	for {
		select {
		case event := <-l.events:
			if err := enc.Encode(event); err != nil {
				l.writeErrors.Add(1)
				enc = json.NewEncoder(l.w)
			}
		case <-l.done:
			for {
				select {
				case event := <-l.events:
					if err := enc.Encode(event); err != nil {
						l.writeErrors.Add(1)
						enc = json.NewEncoder(l.w)
					}
				default:
					return
				}
			}
		}
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()

	close(l.done)
	l.wg.Wait()
}

func (l *Logger) Dropped() int64 {
	return l.dropped.Load()
}

func (l *Logger) WriteErrors() int64 {
	return l.writeErrors.Load()
}

func (l *Logger) Log(event AuditEvent) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		l.dropped.Add(1)
		return
	}

	select {
	case l.events <- event:
	default:
		l.dropped.Add(1)
	}
}
