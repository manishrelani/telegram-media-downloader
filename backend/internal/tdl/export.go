package tdl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

type exportFile struct {
	ID       int64           `json:"id"`
	Messages []exportMessage `json:"messages"`
}

type exportMessage struct {
	ID   int64           `json:"id"`
	Type string          `json:"type"`
	File string          `json:"file"`
	Text string          `json:"text,omitempty"`
	Date int64           `json:"date,omitempty"`
	Raw  json.RawMessage `json:"raw,omitempty"`
}

// ParseExport parses the JSON format emitted by `tdl chat export`. With --with-content,
// the exported message's text becomes the preferred caption/title. Entries without a file
// are ignored because they do not represent a media item for download.
func ParseExport(data []byte) ([]domain.MediaItem, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("tdl export JSON is empty")
	}
	var export exportFile
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse tdl export JSON: %w", err)
	}
	if export.ID == 0 {
		return nil, errors.New("tdl export JSON is missing a non-zero chat ID")
	}
	items := make([]domain.MediaItem, 0, len(export.Messages))
	for _, message := range export.Messages {
		name := strings.TrimSpace(message.File)
		if name == "" {
			continue
		}
		if message.ID <= 0 {
			return nil, fmt.Errorf("media message is missing a positive ID")
		}
		var raw map[string]any
		_ = json.Unmarshal(message.Raw, &raw)
		size, duration, mime, topicID := rawMetadata(raw)
		preview := thumbnailReference(message.Raw)
		version := ""
		if preview != nil {
			version = strconv.FormatInt(preview.ID, 10)
		}
		items = append(items, domain.MediaItem{
			ContentID:        contentIdentity(message.Raw),
			ChatID:           export.ID,
			MessageID:        message.ID,
			Caption:          message.Text,
			Thumbnail:        embeddedThumbnail(raw),
			HasSpoiler:       mediaHasSpoiler(raw),
			Preview:          preview,
			ThumbnailVersion: version,
			OriginalName:     name,
			Date:             message.Date, SizeBytes: size, DurationSeconds: duration, MimeType: mime, TopicID: topicID,
		})
	}
	return items, nil
}

// gotd raw JSON uses Go field names; accept snake_case too for compatible exporters.
func rawField(object map[string]any, name string) any {
	for key, value := range object {
		if strings.EqualFold(strings.ReplaceAll(key, "_", ""), name) {
			return value
		}
	}
	return nil
}
func rawObject(object map[string]any, name string) map[string]any {
	value, _ := rawField(object, name).(map[string]any)
	return value
}
func rawNumber(object map[string]any, name string) float64 {
	value, _ := rawField(object, name).(float64)
	return value
}
func rawMetadata(raw map[string]any) (int64, float64, string, int64) {
	document := rawObject(rawObject(raw, "media"), "document")
	size := int64(rawNumber(document, "size"))
	mime, _ := rawField(document, "mimetype").(string)
	var duration float64
	if attrs, ok := rawField(document, "attributes").([]any); ok {
		for _, attr := range attrs {
			if object, ok := attr.(map[string]any); ok {
				if n := rawNumber(object, "duration"); n > 0 {
					duration = n
				}
			}
		}
	}
	reply := rawObject(raw, "replyto")
	topic := int64(rawNumber(reply, "replytotopid"))
	if topic == 0 {
		if forum, _ := rawField(reply, "forumtopic").(bool); forum {
			topic = int64(rawNumber(reply, "replytomsgid"))
		}
	}
	return size, duration, mime, topic
}

// FilterExport creates a tdl-compatible export containing only the requested media items.
// Raw JSON fields are preserved so newer tdl export fields are not silently discarded.
func FilterExport(data []byte, items []domain.MediaItem) ([]byte, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("tdl export JSON is empty")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse tdl export JSON: %w", err)
	}
	var chatID int64
	if err := json.Unmarshal(root["id"], &chatID); err != nil || chatID == 0 {
		return nil, errors.New("tdl export JSON is missing a non-zero chat ID")
	}
	var messages []json.RawMessage
	if raw := root["messages"]; len(raw) == 0 {
		return nil, errors.New("tdl export JSON is missing messages")
	} else if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("parse tdl export messages: %w", err)
	}
	allowed := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ChatID != chatID {
			return nil, fmt.Errorf("requested media belongs to chat %d, export belongs to chat %d", item.ChatID, chatID)
		}
		allowed[item.Key()] = struct{}{}
	}
	filtered := make([]json.RawMessage, 0, len(allowed))
	for _, rawMessage := range messages {
		var message struct {
			ID   int64  `json:"id"`
			File string `json:"file"`
		}
		if err := json.Unmarshal(rawMessage, &message); err != nil {
			return nil, fmt.Errorf("parse tdl export message: %w", err)
		}
		key := (domain.MediaItem{ChatID: chatID, MessageID: message.ID, OriginalName: strings.TrimSpace(message.File)}).Key()
		if message.ID != 0 && strings.TrimSpace(message.File) != "" {
			if _, ok := allowed[key]; ok {
				filtered = append(filtered, rawMessage)
			}
		}
	}
	encodedMessages, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("encode filtered tdl messages: %w", err)
	}
	root["messages"] = encodedMessages
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode filtered tdl export: %w", err)
	}
	return encoded, nil
}

// Read IDs directly as int64, never through float64 or a thumbnail location.
func contentIdentity(raw json.RawMessage) string {
	var message struct {
		Media struct {
			Document struct{ ID int64 }
			Photo    struct{ ID int64 }
		}
	}
	if json.Unmarshal(raw, &message) != nil {
		return ""
	}
	if id := message.Media.Document.ID; id != 0 {
		return "document:" + strconv.FormatInt(id, 10)
	}
	if id := message.Media.Photo.ID; id != 0 {
		return "photo:" + strconv.FormatInt(id, 10)
	}
	return ""
}
