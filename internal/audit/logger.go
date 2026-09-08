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
	enc := json.NewEncoder(l.w)
	for {
		select {
		case event := <-l.events:
			if err := enc.Encode(event); err != nil {
				l.writeErrors.Add(1)
			}
		case <-l.done:
			for {
				select {
				case event := <-l.events:
					if err := enc.Encode(event); err != nil {
						l.writeErrors.Add(1)
					}
				default:
					l.wg.Done()
					return
				}
			}
		}
	}
}

func (l *Logger) Close() {
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
	select {
	case <-l.done:
		l.dropped.Add(1)
		return
	default:
	}

	select {
	case l.events <- event:
	default:
		l.dropped.Add(1)
	}
}
