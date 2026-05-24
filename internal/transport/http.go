package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"

	// "log/slog"
	"net/http"
)

const defaultMaxBodyBytes int64 = 4 << 20 // 4 MiB

type StreamableHTTP struct {
	handler      MessageHandler
	maxBodyBytes int64
}

type MessageHandler interface {
	Handle(ctx context.Context, msg JSONRPCMessage) (*JSONRPCMessage, error)
}

type HTTPError struct {
	Status  int
	Message string
	Err     error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

type HTTPOption func(*StreamableHTTP) error

func WithMaxBodyBytes(n int64) HTTPOption {
	return func(s *StreamableHTTP) error {
		if n <= 0 {
			return errors.New("max body bytes must be positive")
		}
		s.maxBodyBytes = n
		return nil
	}
}

func NewStreamableHTTP(handler MessageHandler, opts ...HTTPOption) (*StreamableHTTP, error) {
	if handler == nil {
		return nil, errors.New("nil message handler")
	}
	s := &StreamableHTTP{
		handler:      handler,
		maxBodyBytes: defaultMaxBodyBytes,
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}
	if s.maxBodyBytes <= 0 {
		return nil, fmt.Errorf("max body bytes must be positive")
	}
	return s, nil
}

func (s *StreamableHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	default:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *StreamableHTTP) handlePost(w http.ResponseWriter, r *http.Request) {
	var jsonrpcReq JSONRPCMessage
	if err := decodeJSON(w, r, &jsonrpcReq, s.maxBodyBytes); err != nil {
		s.writeError(w, err)
		return
	}
	jsonrpcRes, err := s.handler.Handle(r.Context(), jsonrpcReq)
	if err != nil {
		// TODO: change to json-rpc error response
		s.writeError(w, fmt.Errorf("handle json-rpc request: %w", err))
		return
	}
	//notification
	if jsonrpcRes == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := encodeJSON(w, http.StatusOK, jsonrpcRes); err != nil {
		// TODO: log
		return
	}
}

func (s *StreamableHTTP) writeError(w http.ResponseWriter, err error) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBodyBytes int64) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return unsupportedMediaType("Content-Type must be application/json", nil)
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return unsupportedMediaType("invalid Content-Type", err)
	}

	if mediaType != "application/json" {
		return unsupportedMediaType("Content-Type must be application/json", nil)
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxErr):
			return badRequest("malformed JSON body", err)

		case errors.As(err, &typeErr):
			return badRequest("invalid JSON field type", err)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return badRequest("malformed JSON body", err)

		case errors.As(err, &maxBytesErr):
			return requestTooLarge("request body too large", err)

		case errors.Is(err, io.EOF):
			return badRequest("request body is required", err)

		default:
			return badRequest("invalid JSON body", nil)
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("request body must contain a single JSON value", err)
	}
	return nil
}

func encodeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json response: %w", err)
	}
	return nil
}

func badRequest(message string, err error) error {
	return &HTTPError{
		Status:  http.StatusBadRequest,
		Message: message,
		Err:     err,
	}
}

func requestTooLarge(message string, err error) error {
	return &HTTPError{
		Status:  http.StatusRequestEntityTooLarge,
		Message: message,
		Err:     err,
	}
}

func unsupportedMediaType(message string, err error) error {
	return &HTTPError{
		Status:  http.StatusUnsupportedMediaType,
		Message: message,
		Err:     err,
	}
}
