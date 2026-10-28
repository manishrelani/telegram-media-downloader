package protocol

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type testHandler struct{ actions []string }

func (h *testHandler) Handle(_ context.Context, r Request, emit func(Message)) {
	h.actions = append(h.actions, r.Action)
	emit(Message{Event: "result", Data: map[string]string{"action": r.Action}})
}

func TestServeContinuesAfterMalformedJSON(t *testing.T) {
	h := &testHandler{}
	input := "not-json\n{\"version\":1,\"id\":\"r1\",\"action\":\"health\"}\n"
	var output strings.Builder
	if err := Serve(context.Background(), strings.NewReader(input), &output, h); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2: %s", len(lines), output.String())
	}
	var first Message
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Error == nil || first.Error.Code != "invalid_json" {
		t.Fatalf("unexpected first response: %+v", first)
	}
	if len(h.actions) != 1 || h.actions[0] != "health" {
		t.Fatalf("handler actions = %v", h.actions)
	}
}

func TestServeRejectsUnknownProtocolVersion(t *testing.T) {
	h := &testHandler{}
	var output strings.Builder
	input := "{\"version\":99,\"id\":\"r1\",\"action\":\"health\"}\n"
	if err := Serve(context.Background(), strings.NewReader(input), &output, h); err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error == nil || got.Error.Code != "unsupported_protocol_version" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if len(h.actions) != 0 {
		t.Fatalf("unsupported request reached handler: %v", h.actions)
	}
}

func TestServeRejectsMissingIDOrAction(t *testing.T) {
	for _, input := range []string{
		"{\"version\":1,\"action\":\"health\"}\n",
		"{\"version\":1,\"id\":\"r1\"}\n",
	} {
		h := &testHandler{}
		var output strings.Builder
		if err := Serve(context.Background(), strings.NewReader(input), &output, h); err != nil {
			t.Fatal(err)
		}
		var got Message
		if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &got); err != nil {
			t.Fatal(err)
		}
		if got.Error == nil || got.Error.Code != "invalid_request" {
			t.Fatalf("unexpected response: %+v", got)
		}
		if len(h.actions) != 0 {
			t.Fatalf("invalid request reached handler: %v", h.actions)
		}
	}
}
