package application

import (
	"context"
	"encoding/json"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const chatManifestName = ".telegram-media-manager-manifest.json"

// Keep the original friendly directory after display-name edits. Manifests also
// identify folders when job history is unavailable; nothing is moved or merged.
func (s *Service) resolveChatDirectories(ctx context.Context, destination string, chat domain.Chat) (domain.ChatDirectories, error) {
	root, err := filepath.Abs(strings.TrimSpace(destination))
	if err != nil {
		return domain.ChatDirectories{}, err
	}
	label := strings.TrimSpace(chat.Name)
	if label == "" {
		label = strings.TrimSpace(chat.Username)
	}
	if label == "" {
		label = "Telegram Chat"
	}
	proposed, err := domain.BuildChatDirectories(root, fmt.Sprintf("%s [%d]", label, chat.ID))
	if err != nil {
		return domain.ChatDirectories{}, err
	}
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	candidates := map[string]bool{}
	key := fmt.Sprintf("%s:%d", root, chat.ID)
	if reserved := s.chatRoots[key]; reserved != "" {
		candidates[reserved] = true
	}
	jobs, err := s.Jobs.List(ctx)
	if err != nil {
		return domain.ChatDirectories{}, fmt.Errorf("resolve saved chat folder: %w", err)
	}
	for _, job := range jobs {
		if job.Chat.ID != chat.ID || strings.TrimSpace(job.Destination) == "" {
			continue
		}
		path, err := filepath.Abs(job.Destination)
		if err == nil && filepath.Dir(path) == root {
			candidates[path] = true
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return domain.ChatDirectories{}, storageFailure(fmt.Errorf("read save folder: %w", err))
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, readErr := os.ReadFile(filepath.Join(path, chatManifestName))
		likely := path == proposed.Root || strings.HasSuffix(entry.Name(), fmt.Sprintf(" [%d]", chat.ID)) || candidates[path]
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			if likely {
				return domain.ChatDirectories{}, storageFailure(fmt.Errorf("read saved chat manifest: %w", readErr))
			}
			continue
		}
		var identity struct {
			ChatID int64 `json:"chatId"`
		}
		if err := json.Unmarshal(data, &identity); err != nil {
			if likely {
				return domain.ChatDirectories{}, storageFailure(fmt.Errorf("read saved chat identity: %w", err))
			}
			continue
		}
		if identity.ChatID == chat.ID {
			candidates[path] = true
		} else if candidates[path] || path == proposed.Root {
			return domain.ChatDirectories{}, fmt.Errorf("saved folder belongs to another chat; choose a different save folder")
		}
	}
	if len(candidates) > 1 {
		return domain.ChatDirectories{}, fmt.Errorf("multiple saved folders exist for chat %d in this destination; resolve the duplicate folders before downloading", chat.ID)
	}
	selected := proposed.Root
	for path := range candidates {
		selected = path
	}
	if info, err := os.Lstat(selected); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return domain.ChatDirectories{}, fmt.Errorf("saved chat folder is not a regular directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.ChatDirectories{}, storageFailure(err)
	}
	if s.chatRoots == nil {
		s.chatRoots = map[string]string{}
	}
	s.chatRoots[key] = selected
	return domain.ChatDirectories{Root: selected, Videos: filepath.Join(selected, "Videos"), Documents: filepath.Join(selected, "Documents")}, nil
}
