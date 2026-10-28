package tdl

import (
	"bytes"
	"encoding/base64"
	"image/jpeg"
)

func mediaHasSpoiler(raw map[string]any) bool {
	spoiler, _ := rawField(rawObject(raw, "media"), "spoiler").(bool)
	return spoiler
}

// Embedded previews arrive in the existing raw export, with no extra Telegram
// requests or full-video transfer. Ignore vector previews and malformed images.
func embeddedThumbnail(raw map[string]any) string {
	media := rawObject(raw, "media")
	if spoiler, _ := rawField(media, "spoiler").(bool); spoiler {
		return ""
	}
	document := rawObject(media, "document")
	thumbs, _ := rawField(document, "thumbs").([]any)
	var best []byte
	var area int
	for _, value := range thumbs {
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := rawField(object, "type").(string)
		if kind == "j" {
			continue
		}
		encoded, _ := rawField(object, "bytes").(string)
		if len(encoded) == 0 || len(encoded) > 44000 {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(data) > 32768 {
			continue
		}
		if kind == "i" {
			if len(data) < 4 || data[0] != 1 || data[1] == 0 || data[2] == 0 {
				continue
			}
			expanded := append([]byte(nil), thumbnailJPEGHeader...)
			expanded[164] = data[1]
			expanded[166] = data[2]
			expanded = append(expanded, data[3:]...)
			expanded = append(expanded, 0xff, 0xd9)
			data = expanded
		}
		config, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 512 || config.Height > 512 {
			continue
		}
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			continue
		}
		pixels := config.Width * config.Height
		if pixels > area {
			best = data
			area = pixels
		}
	}
	return base64.StdEncoding.EncodeToString(best)
}
