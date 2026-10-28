package domain

const ManifestVersion = 1

type Manifest struct {
	Version  int                      `json:"version"`
	ChatID   int64                    `json:"chatId"`
	ChatName string                   `json:"chatName"`
	Items    map[string]ManifestEntry `json:"items"`
}

type ManifestEntry struct {
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	ContentID    string `json:"contentId,omitempty"`
	ChatID       int64  `json:"chatId"`
	MessageID    int64  `json:"messageId"`
	OriginalName string `json:"originalName"`
	Caption      string `json:"caption,omitempty"`
	SavedName    string `json:"savedName"`
	SavedPath    string `json:"savedPath,omitempty"`
	Status       string `json:"status"`
}

func NewManifest(chatID int64, chatName string) *Manifest {
	return &Manifest{Version: ManifestVersion, ChatID: chatID, ChatName: chatName, Items: make(map[string]ManifestEntry)}
}

func (m *Manifest) IsCompleted(item MediaItem) bool {
	if m == nil {
		return false
	}
	entry, ok := m.Items[item.Key()]
	return ok && entry.Status == "completed"
}

func (m *Manifest) MarkCompleted(item MediaItem, savedName string) {
	if m == nil {
		return
	}
	if m.Items == nil {
		m.Items = make(map[string]ManifestEntry)
	}
	m.Items[item.Key()] = ManifestEntry{
		ChatID: item.ChatID, MessageID: item.MessageID, OriginalName: item.OriginalName,
		SizeBytes: item.SizeBytes, ContentID: item.ContentID, Caption: item.Caption, SavedName: savedName, Status: "completed",
	}
}

func (m *Manifest) CompletedCount() int {
	if m == nil {
		return 0
	}
	count := 0
	for _, entry := range m.Items {
		if entry.Status == "completed" {
			count++
		}
	}
	return count
}

// MarkMoving records the intended final filename before the staging file is moved.
// If the process crashes between the move and MarkCompleted, startup can reconcile
// this journal entry by checking whether the final file exists.
func (m *Manifest) MarkMoving(item MediaItem, savedName string) {
	m.MarkCompleted(item, savedName)
	entry := m.Items[item.Key()]
	entry.Status = "moving"
	m.Items[item.Key()] = entry
}
