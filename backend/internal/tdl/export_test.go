package tdl

import (
	"encoding/json"
	"reflect"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestParseExportMapsMediaMessagesAndCaptions(t *testing.T) {
	input := []byte(`{
  "id": 12345,
  "messages": [
    {"id": 26, "type": "message", "file": "lesson.mp4", "text": "01. Introduction to GenAI"},
    {"id": 25, "type": "message", "file": "notes.pdf", "text": "RAG Architecture Notes"},
    {"id": 24, "type": "message", "file": "untitled.mkv"},
    {"id": 23, "type": "message", "text": "ordinary text message"},
    {"id": 22, "type": "message", "file": ""}
  ]
}`)
	got, err := ParseExport(input)
	if err != nil {
		t.Fatalf("ParseExport() error = %v", err)
	}
	want := []domain.MediaItem{
		{ChatID: 12345, MessageID: 26, Caption: "01. Introduction to GenAI", OriginalName: "lesson.mp4"},
		{ChatID: 12345, MessageID: 25, Caption: "RAG Architecture Notes", OriginalName: "notes.pdf"},
		{ChatID: 12345, MessageID: 24, OriginalName: "untitled.mkv"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseExport() = %#v, want %#v", got, want)
	}
}

func TestParseExportEmptyMessagesReturnsEmptySlice(t *testing.T) {
	got, err := ParseExport([]byte(`{"id":12345,"messages":[]}`))
	if err != nil {
		t.Fatalf("ParseExport() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("ParseExport() = %#v, want non-nil empty slice", got)
	}
}

func TestParseExportRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseExport([]byte(`{"id":`)); err == nil {
		t.Fatal("ParseExport() error = nil, want parse error")
	}
}

func TestParseExportRejectsMissingChatID(t *testing.T) {
	if _, err := ParseExport([]byte(`{"messages":[{"id":1,"file":"a.mp4"}]}`)); err == nil {
		t.Fatal("ParseExport() error = nil, want missing chat ID error")
	}
}

func TestFilterExportKeepsRequestedMediaAndPreservesUnknownFields(t *testing.T) {
	input := []byte(`{"id":12345,"meta":{"futureField":true},"messages":[{"id":26,"type":"message","file":"lesson.mp4","text":"Lesson","extra":{"keep":1}},{"id":25,"type":"message","file":"notes.pdf","text":"Notes"},{"id":24,"type":"message","text":"No media"}]}`)
	items := []domain.MediaItem{{ChatID: 12345, MessageID: 26, OriginalName: "lesson.mp4", Caption: "Lesson"}}
	filtered, err := FilterExport(input, items)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseExport(filtered)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.MediaItem{{ChatID: 12345, MessageID: 26, OriginalName: "lesson.mp4", Caption: "Lesson"}}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("FilterExport parsed = %#v, want %#v", parsed, want)
	}
	var root map[string]any
	if err := json.Unmarshal(filtered, &root); err != nil {
		t.Fatal(err)
	}
	if root["meta"] == nil {
		t.Fatalf("unknown root field was lost: %s", filtered)
	}
	messages := root["messages"].([]any)
	message := messages[0].(map[string]any)
	if message["extra"] == nil {
		t.Fatalf("unknown message field was lost: %s", filtered)
	}
}

func TestFilterExportReturnsEmptyMessagesWhenNoItemsRequested(t *testing.T) {
	filtered, err := FilterExport([]byte(`{"id":12345,"messages":[{"id":26,"type":"message","file":"lesson.mp4"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	items, err := ParseExport(filtered)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("filtered items = %#v, want none", items)
	}
}
