package tdl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"image"
	"image/jpeg"
	"testing"
)

func TestEmbeddedThumbnails(t *testing.T) {
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 16, 9)), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		thumbs any
		want   bool
	}{
		{"cached", []any{map[string]any{"Type": "s", "Bytes": base64.StdEncoding.EncodeToString(data.Bytes())}}, true},
		{"corrupt", []any{map[string]any{"Type": "s", "Bytes": "broken!"}}, false},
		{"vector", []any{map[string]any{"Type": "j", "Bytes": base64.StdEncoding.EncodeToString(data.Bytes())}}, false},
		{"absent", nil, false},
		{"oversize", []any{map[string]any{"Type": "s", "Bytes": base64.StdEncoding.EncodeToString(make([]byte, 40000))}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(map[string]any{"id": 123, "messages": []any{map[string]any{"id": 7, "file": "a.mp4", "raw": map[string]any{"Media": map[string]any{"Document": map[string]any{"Thumbs": tc.thumbs}}}}}})
			items, err := ParseExport(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if (items[0].Thumbnail != "") != tc.want {
				t.Fatalf("thumbnail present=%v", items[0].Thumbnail != "")
			}
			if tc.want {
				decoded, _ := base64.StdEncoding.DecodeString(items[0].Thumbnail)
				if _, err := jpeg.Decode(bytes.NewReader(decoded)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestImageAudioFileTypes(t *testing.T) {
	for name, want := range map[string]string{"photo.HEIC": "image", "voice.ogg": "audio", "code.ts": "code"} {
		if got := domain.FileType(name); got != want {
			t.Fatalf("%s: %s", name, got)
		}
	}
}

func TestStrippedThumbnailAndUnsafeDimensions(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 16, 9)), &jpeg.Options{Quality: 20}); err != nil {
		t.Fatal(err)
	}
	source := encoded.Bytes()
	scan := bytes.Index(source, []byte{0xff, 0xda})
	if scan < 0 {
		t.Fatal("missing scan")
	}
	scanLength := int(source[scan+2])*256 + int(source[scan+3])
	stripped := append([]byte{1, 9, 16}, source[scan+2+scanLength:len(source)-2]...)
	for _, tc := range []struct {
		name string
		data []byte
		kind string
		want bool
	}{
		{"stripped", stripped, "i", true}, {"short", []byte{1, 2}, "i", false}, {"bad-version", []byte{2, 9, 16, 0}, "i", false}, {"zero-dimension", []byte{1, 0, 16, 0}, "i", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{"Media": map[string]any{"Document": map[string]any{"Thumbs": []any{map[string]any{"Type": tc.kind, "Bytes": base64.StdEncoding.EncodeToString(tc.data)}}}}}
			result := embeddedThumbnail(raw)
			if (result != "") != tc.want {
				t.Fatalf("present=%v", result != "")
			}
			raw["Media"].(map[string]any)["Spoiler"] = true
			if embeddedThumbnail(raw) != "" {
				t.Fatal("spoiler preview was exposed")
			}
		})
	}
	var large bytes.Buffer
	if err := jpeg.Encode(&large, image.NewRGBA(image.Rect(0, 0, 513, 1)), nil); err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{"media": map[string]any{"document": map[string]any{"thumbs": []any{map[string]any{"type": "s", "bytes": base64.StdEncoding.EncodeToString(large.Bytes())}}}}}
	if embeddedThumbnail(raw) != "" {
		t.Fatal("unbounded preview accepted")
	}
}
