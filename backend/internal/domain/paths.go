package domain

import (
	"errors"
	"path/filepath"
	"strings"
)

type ChatDirectories struct {
	Root      string `json:"root"`
	Videos    string `json:"videos"`
	Documents string `json:"documents"`
}

// BuildChatDirectories creates a safe, deterministic per-chat directory layout. It only
// computes paths; it does not touch the filesystem. The adapter must create/check them.
func BuildChatDirectories(destinationRoot, chatName string) (ChatDirectories, error) {
	root := strings.TrimSpace(destinationRoot)
	if root == "" {
		return ChatDirectories{}, errors.New("destination root is required")
	}
	name := sanitizeFilenameBase(chatName)
	if name == "" {
		name = "Telegram Chat"
	}
	if isWindowsReservedName(name) {
		name = "_" + name
	}
	chatRoot := filepath.Join(root, name)
	return ChatDirectories{
		Root:      chatRoot,
		Videos:    filepath.Join(chatRoot, "Videos"),
		Documents: filepath.Join(chatRoot, "Documents"),
	}, nil
}
