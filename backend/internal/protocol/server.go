package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

const maxRequestBytes = 1024 * 1024

type Handler interface {
	Handle(context.Context, Request, func(Message))
}

// Serve reads one JSON request per line and emits zero or more JSON events per request.
// Malformed requests are reported without terminating the process.
func Serve(ctx context.Context, in io.Reader, out io.Writer, h Handler) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), maxRequestBytes)
	writer := bufio.NewWriter(out)
	var writeMu sync.Mutex
	write := func(m Message) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if m.Version == 0 {
			m.Version = Version
		}
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("encode protocol message: %w", err)
		}
		if _, err = writer.Write(append(b, '\n')); err != nil {
			return err
		}
		return writer.Flush()
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if werr := write(Message{Event: "error", Error: &Error{Code: "invalid_json", Message: "Request must be valid JSON."}}); werr != nil {
				return werr
			}
			continue
		}
		if req.Version != Version {
			if err := write(Message{ID: req.ID, Event: "error", Error: &Error{Code: "unsupported_protocol_version", Message: fmt.Sprintf("Protocol version %d is not supported; expected %d.", req.Version, Version)}}); err != nil {
				return err
			}
			continue
		}
		if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Action) == "" {
			if err := write(Message{ID: req.ID, Event: "error", Error: &Error{Code: "invalid_request", Message: "Request requires non-empty id and action."}}); err != nil {
				return err
			}
			continue
		}
		h.Handle(ctx, req, func(m Message) {
			if m.ID == "" {
				m.ID = req.ID
			}
			_ = write(m)
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read protocol input: %w", err)
	}
	return nil
}
