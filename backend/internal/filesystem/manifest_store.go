package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
)

var _ ports.ManifestStore = JSONManifestStore{}

// JSONManifestStore persists one manifest per chat using a write-temp, sync, rename flow.
// Manifest files contain media names and local paths, never Telegram credentials.
type JSONManifestStore struct{}

func (JSONManifestStore) Load(ctx context.Context, path string, chatID int64, chatName string) (*domain.Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("manifest path is required")
	}
	if chatID == 0 {
		return nil, errors.New("manifest chat ID is required")
	}
	boundary, name, err := manifestRoot(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return domain.NewManifest(chatID, chatName), nil
	}
	if err != nil {
		return nil, err
	}
	defer boundary.Close()
	data, err := boundary.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return domain.NewManifest(chatID, chatName), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest domain.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Version != domain.ManifestVersion {
		return nil, fmt.Errorf("unsupported manifest version %d (supported: %d)", manifest.Version, domain.ManifestVersion)
	}
	if manifest.ChatID != chatID {
		return nil, fmt.Errorf("manifest belongs to chat %d, not selected chat %d", manifest.ChatID, chatID)
	}
	if manifest.Items == nil {
		manifest.Items = make(map[string]domain.ManifestEntry)
	}
	if strings.TrimSpace(manifest.ChatName) == "" {
		manifest.ChatName = chatName
	}
	return &manifest, nil
}

func (JSONManifestStore) Save(ctx context.Context, path string, manifest *domain.Manifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("manifest path is required")
	}
	if manifest == nil {
		return errors.New("manifest is required")
	}
	if manifest.Version != domain.ManifestVersion {
		return fmt.Errorf("unsupported manifest version %d (supported: %d)", manifest.Version, domain.ManifestVersion)
	}
	if manifest.ChatID == 0 {
		return errors.New("manifest chat ID is required")
	}
	copyOfManifest := *manifest
	if copyOfManifest.Items == nil {
		copyOfManifest.Items = make(map[string]domain.ManifestEntry)
	}
	data, err := json.MarshalIndent(copyOfManifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	boundary, name, err := manifestRoot(path, true)
	if err != nil {
		return err
	}
	defer boundary.Close()
	temporary := ".manifest-" + rand.Text() + ".tmp"
	file, err := boundary.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer boundary.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure temporary manifest: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary manifest: %w", err)
	}
	if err := replaceManifestRoot(boundary, temporary, name); err != nil {
		return fmt.Errorf("replace manifest: %w", err)
	}
	directory, openErr := boundary.Open(".")
	if openErr == nil {
		_ = directory.Sync()
		directory.Close()
	}
	return nil
}

func manifestRoot(path string, create bool) (*os.Root, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	directory := filepath.Dir(absolute)
	anchor := filepath.Dir(directory)
	if create {
		if err := os.MkdirAll(anchor, 0700); err != nil {
			return nil, "", err
		}
	}
	parent, rel, err := safefs.Open(anchor, directory)
	if err != nil {
		return nil, "", err
	}
	defer parent.Close()
	if create {
		if err := parent.MkdirAll(rel, 0700); err != nil {
			return nil, "", err
		}
	}
	boundary, err := parent.OpenRoot(rel)
	if err != nil {
		return nil, "", err
	}
	name := filepath.Base(absolute)
	if err := safefs.Check(boundary, name); err != nil {
		boundary.Close()
		return nil, "", err
	}
	return boundary, name, nil
}

func replaceManifestRoot(root *os.Root, tempPath, targetPath string) error {
	if runtime.GOOS != "windows" {
		return root.Rename(tempPath, targetPath)
	}
	backupPath := targetPath + ".bak"
	_ = root.Remove(backupPath)
	if _, err := root.Stat(targetPath); err == nil {
		if err := root.Rename(targetPath, backupPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := root.Rename(tempPath, targetPath); err != nil {
		if _, backupErr := root.Stat(backupPath); backupErr == nil {
			_ = root.Rename(backupPath, targetPath)
		}
		return err
	}
	_ = root.Remove(backupPath)
	return nil
}
