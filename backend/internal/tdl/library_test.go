package tdl

import (
	"reflect"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestScopedExportArguments(t *testing.T) {
	args, err := BuildMediaExportArgs(domain.MediaQuery{Chat: domain.Chat{ID: 123}, Topic: &domain.Topic{ID: 42, Title: "Backend"}, Limit: 101}, "/tmp/export.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"chat", "export", "-c", "123", "--with-content", "--raw", "-o", "/tmp/export.json", "--topic", "42", "-T", "last", "-i", "101"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v", args)
	}
}

func TestExportMetadataAndIdentity(t *testing.T) {
	items, err := ParseExport([]byte(`{"id":123,"messages":[{"id":7,"file":"a.mp4","text":"Lesson","date":1700000000,"raw":{"Media":{"Document":{"Size":1000,"MimeType":"video/mp4","Attributes":[{"Duration":12.5}]}}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SizeBytes != 1000 || items[0].Date != 1700000000 || items[0].DurationSeconds != 12.5 {
		t.Fatalf("items=%+v", items)
	}
}
