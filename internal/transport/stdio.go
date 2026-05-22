package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

const MaxMCPMessageBytes = 16 << 20

type StdioTransport struct {
	command string
	args    []string

	stdin   io.WriteCloser
	pending map[uint64]chan *Message

	pendingMu sync.RWMutex
	writeMu   sync.Mutex

	closed   bool
	closedMu sync.RWMutex

	capabilities *Capabilities
	nextID       atomic.Uint64
	cmd          *exec.Cmd
}

func NewStdioTransport(command string, args ...string) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
		pending: make(map[uint64]chan *Message),
	}
}

// Connect starts the upstream MCP server subprocess and initializes the stdio pipes.
//
// Connect only establishes the transport-level connection. It does not perform
// the MCP initialize handshake; upstream session lifecycle is owned by the manager.
func (t *StdioTransport) Connect(ctx context.Context) error {
	cmd := exec.Command(t.command, t.args...)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("initializing stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("initializing stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting subprocess: %w", err)
	}

	t.stdin = stdinPipe
	t.cmd = cmd

	go t.dispatchLoop(ctx, stdoutPipe)

	go func() {
		cmd.Wait()
	}()

	return nil
}

func (t *StdioTransport) dispatchLoop(ctx context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), MaxMCPMessageBytes)
	for scanner.Scan() {
		data := make([]byte, len(scanner.Bytes()))
		copy(data, scanner.Bytes())
		mcpMessage := Message{
			Raw: data,
		}
		msg, err := mcpMessage.Parsed()
		if err != nil {
			// do something
		}

		if msg != nil {
			var msgID uint64
			err := json.Unmarshal(*msg.ID, &msgID)
			if err != nil {
				// should discard the message. upstream server sent a non integer id
			}
			t.pendingMu.Lock()
			ch, ok := t.pending[msgID]
			if ok {
				delete(t.pending, msgID)
			}
			t.pendingMu.Unlock()
			ch <- &mcpMessage
		}
	}
}

// Send writes a raw JSON-RPC request to the upstream server and waits for the
// matching JSON-RPC response.
//
// The request must contain a JSON-RPC id. Send registers the request as pending
// before writing it so responses can be correlated by id. It returns when the
// matching response is received, the context is canceled, or the transport fails.
func (t *StdioTransport) Send(ctx context.Context, msg *Message) (*Message, error)

func (t *StdioTransport) Capabilities() *Capabilities

// Close shuts down the stdio transport and releases all subprocess resources.
//
// Close is idempotent. It prevents future sends, unblocks all pending requests,
// closes stdin, terminates the subprocess if it is still running, and waits for
// transport goroutines to exit where applicable.
func (t *StdioTransport) Close() error
