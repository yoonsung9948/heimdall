package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestLogger_WritesEventAsJSON(t *testing.T) {
	var buf bytes.Buffer
	bufferSize := 1
	logger := NewLogger(&buf, bufferSize)
	timestamp := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)

	event := AuditEvent{
		Timestamp:      timestamp,
		UserName:       "alice",
		ClientName:     "cli-1",
		ToolName:       "docs.search",
		ServerName:     "docs",
		Groups:         []string{"admins"},
		DecisionAllow:  true,
		DecisionReason: "allowed by matching rule",
		RuleID:         "allow-admins",
	}
	logger.Log(event)
	logger.Close()
	var got AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, event) {
		t.Errorf("event = %+v, want %+v", got, event)
	}
}
func TestLogger_ConcurrentLogsWithoutOverflow(t *testing.T) {
	var buf bytes.Buffer
	timestamp := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)
	events := []AuditEvent{
		{
			Timestamp:      timestamp,
			UserName:       "alice",
			ClientName:     "cli-1",
			ToolName:       "docs.search",
			ServerName:     "docs",
			Groups:         []string{"admins"},
			DecisionAllow:  true,
			DecisionReason: "allowed by matching rule",
			RuleID:         "allow-admins",
		},
		{
			Timestamp:      timestamp,
			UserName:       "alice",
			ClientName:     "cli-2",
			ToolName:       "docs.search",
			ServerName:     "docs",
			Groups:         []string{"engineering"},
			DecisionAllow:  true,
			DecisionReason: "allowed by matching rule",
			RuleID:         "allow-engineering",
		},
	}

	logger := NewLogger(&buf, len(events))
	var wg sync.WaitGroup
	start := make(chan struct{})

	for _, event := range events {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start
			logger.Log(event)
		}()
	}

	close(start)
	wg.Wait()
	logger.Close()

	if got := logger.Dropped(); got != 0 {
		t.Fatalf("Dropped() = %d, want 0", got)
	}
	if got := logger.WriteErrors(); got != 0 {
		t.Errorf("WriteErrors() = %d, want 0", got)
	}

	var got []AuditEvent

	dec := json.NewDecoder(&buf)
	for {
		var event AuditEvent
		err := dec.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		got = append(got, event)
	}

	if len(got) != len(events) {
		t.Fatalf("got %d events, want %d", len(got), len(events))
	}

	gotByClient := make(map[string]AuditEvent, len(got))
	for _, event := range got {
		gotByClient[event.ClientName] = event
	}

	for _, want := range events {
		got, ok := gotByClient[want.ClientName]
		if !ok {
			t.Errorf("missing event for client %q", want.ClientName)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("event for %q = %+v, want %+v", want.ClientName, got, want)
		}
	}
}

func TestLogger_DropsEventsWhenQueueFull(t *testing.T) {
	writer := &blockingWriter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	logger := NewLogger(writer, 1)

	var finishOnce sync.Once
	closeFinished := make(chan struct{})
	finish := func() {
		finishOnce.Do(func() {
			close(writer.release)
			go func() {
				logger.Close()
				close(closeFinished)
			}()
		})
		select {
		case <-closeFinished:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for logger shutdown")
		}
	}
	t.Cleanup(finish)

	eventA := AuditEvent{ClientName: "A"}
	eventB := AuditEvent{ClientName: "B"}
	eventC := AuditEvent{ClientName: "C"}

	logger.Log(eventA)
	select {
	case <-writer.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event A to reach the writer")
	}

	// A is blocked in the writer, so B fills the only queue slot
	logger.Log(eventB)
	logger.Log(eventC)
	if got := logger.Dropped(); got != 1 {
		t.Errorf("Dropped() = %d, want 1", got)
	}

	finish()
	if got := logger.WriteErrors(); got != 0 {
		t.Errorf("WriteErrors() = %d, want 0", got)
	}

	var got []AuditEvent
	decoder := json.NewDecoder(&writer.buf)
	for {
		var event AuditEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode logged event: %v", err)
		}
		got = append(got, event)
	}

	want := []AuditEvent{eventA, eventB}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("logged events = %+v, want %+v", got, want)
	}
}

func TestLogger_CloseDrainsQueuedEventsAndRejectsNewLogs(t *testing.T) {
	writer := &blockingWriter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	logger := NewLogger(writer, 2)

	var releaseOnce, closeOnce sync.Once
	closeFinished := make(chan struct{})

	releaseWriter := func() {
		releaseOnce.Do(func() { close(writer.release) })
	}
	startClose := func() {
		closeOnce.Do(func() {
			go func() {
				logger.Close()
				close(closeFinished)
			}()
		})
	}
	wait := func(ch <-chan struct{}, description string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s", description)
		}
	}

	t.Cleanup(func() {
		releaseWriter()
		startClose()
		wait(closeFinished, "logger shutdown")
	})

	eventA := AuditEvent{ClientName: "A"}
	eventB := AuditEvent{ClientName: "B"}
	eventC := AuditEvent{ClientName: "C"}

	logger.Log(eventA)
	wait(writer.started, "event A to reach the writer")
	logger.Log(eventB)

	startClose()
	wait(logger.done, "shutdown acceptance cutoff")

	logger.Log(eventC)

	releaseWriter()
	wait(closeFinished, "logger to drain and close")

	var got []AuditEvent
	decoder := json.NewDecoder(&writer.buf)
	for {
		var event AuditEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode logged event: %v", err)
		}
		got = append(got, event)
	}

	want := []AuditEvent{eventA, eventB}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("logged events = %+v, want %+v", got, want)
	}
	if got := logger.Dropped(); got != 1 {
		t.Errorf("Dropped() = %d, want 1", got)
	}
	if got := logger.WriteErrors(); got != 0 {
		t.Errorf("WriteErrors() = %d, want 0", got)
	}
}

func TestLogger_ConcurrentLogAndClose(t *testing.T) {
	const eventCount = 100
	var buf bytes.Buffer
	logger := NewLogger(&buf, eventCount)
	wantByClient := make(map[string]AuditEvent, eventCount)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < eventCount; i++ {
		event := AuditEvent{ClientName: fmt.Sprintf("client-%d", i)}
		wantByClient[event.ClientName] = event
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			logger.Log(event)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		logger.Close()
	}()

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	close(start)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent logging and shutdown")
	}

	seen := make(map[string]bool, eventCount)
	written := 0
	decoder := json.NewDecoder(&buf)
	for {
		var event AuditEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode logged event: %v", err)
		}
		written++
		want, ok := wantByClient[event.ClientName]
		if !ok {
			t.Errorf("unexpected event for client %q", event.ClientName)
		} else if !reflect.DeepEqual(event, want) {
			t.Errorf("event = %+v, want %+v", event, want)
		}
		if seen[event.ClientName] {
			t.Errorf("duplicate event for client %q", event.ClientName)
		}
		seen[event.ClientName] = true
	}
	if dropped := logger.Dropped(); int64(written)+dropped != eventCount {
		t.Errorf("written %d + dropped %d != submitted %d", written, dropped, eventCount)
	}
	if queued := len(logger.events); queued != 0 {
		t.Errorf("queued events after shutdown = %d, want 0", queued)
	}
	if got := logger.WriteErrors(); got != 0 {
		t.Errorf("WriteErrors() = %d, want 0", got)
	}
}

func TestLogger_WriteErrorIsCountedAndProcessingContinues(t *testing.T) {
	writer := &failOnceWriter{err: errors.New("temporary write failure")}
	logger := NewLogger(writer, 2)
	first := AuditEvent{ClientName: "first"}
	second := AuditEvent{ClientName: "second"}

	logger.Log(first)
	logger.Log(second)
	logger.Close()

	if got := logger.WriteErrors(); got != 1 {
		t.Errorf("WriteErrors() = %d, want 1", got)
	}
	if got := logger.Dropped(); got != 0 {
		t.Errorf("Dropped() = %d, want 0", got)
	}
	if writer.calls != 2 {
		t.Errorf("writer calls = %d, want 2", writer.calls)
	}

	var got AuditEvent
	if err := json.Unmarshal(writer.buf.Bytes(), &got); err != nil {
		t.Fatalf("decode surviving event: %v", err)
	}
	if !reflect.DeepEqual(got, second) {
		t.Errorf("written event = %+v, want %+v", got, second)
	}
}

type failOnceWriter struct {
	buf   bytes.Buffer
	err   error
	calls int
}

func (w *failOnceWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		return 0, w.err
	}
	return w.buf.Write(p)
}

type blockingWriter struct {
	buf     bytes.Buffer
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.started)
		<-w.release
	})
	return w.buf.Write(p)
}
