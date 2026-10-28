package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type thumbnailProvider interface {
	StreamThumbnails(context.Context, []domain.PreviewRequest, func(string, []byte)) error
}

func (s *Service) cancelPreviews() {
	s.previewMu.Lock()
	if s.previewCancel != nil {
		s.previewCancel()
	}
	s.previewMu.Unlock()
}
func validPreview(data []byte) bool {
	if len(data) == 0 || len(data) > 256*1024 {
		return false
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1280 || config.Height > 1280 {
		return false
	}
	_, err = jpeg.Decode(bytes.NewReader(data))
	return err == nil
}
func (s *Service) RequestThumbnails(parent context.Context, snapshotID string, keys []string, emit func(map[string]string)) (bool, error) {
	if len(keys) == 0 || len(keys) > 12 {
		return false, fmt.Errorf("request 1–12 visible thumbnails")
	}
	s.libraryMu.Lock()
	snapshot, exists := s.snapshots[snapshotID]
	s.libraryMu.Unlock()
	if !exists {
		return false, fmt.Errorf("media list expired; refresh the chat")
	}
	members := map[string]domain.MediaItem{}
	for _, item := range snapshot.Page.Items {
		members[item.Key()] = item
	}
	for _, key := range keys {
		if _, ok := members[key]; !ok || len(key) > 2048 {
			return false, fmt.Errorf("thumbnail does not belong to this snapshot")
		}
	}
	provider, ok := s.Telegram.(thumbnailProvider)
	if !ok || s.PreviewDirectory == "" {
		return false, nil
	}
	requests := []domain.PreviewRequest{}
	paths := map[string]string{}
	seen := map[string]bool{}
	publish := func(key string, data []byte) {
		if parent.Err() != nil {
			return
		}
		image := base64.StdEncoding.EncodeToString(data)
		s.libraryMu.Lock()
		if current, ok := s.snapshots[snapshotID]; ok {
			current.Page.Items = append([]domain.MediaItem(nil), current.Page.Items...)
			for i := range current.Page.Items {
				if current.Page.Items[i].Key() == key {
					current.Page.Items[i].Thumbnail = image
				}
			}
			s.snapshots[snapshotID] = current
		}
		s.libraryMu.Unlock()
		emit(map[string]string{key: image})
	}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		item := members[key]
		if item.HasSpoiler || item.Preview == nil || domain.FileType(item.OriginalName) != "video" {
			continue
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", key, item.Preview.ID)))
		path := filepath.Join(s.PreviewDirectory, fmt.Sprintf("%x.jpg", sum))
		paths[key] = path
		if info, err := os.Stat(path); err == nil && info.Size() <= 256*1024 {
			if data, err := os.ReadFile(path); err == nil && validPreview(data) {
				publish(key, data)
				continue
			}
		}
		requests = append(requests, domain.PreviewRequest{Key: key, Reference: *item.Preview})
	}
	if len(requests) == 0 {
		return false, nil
	}
	ctx, release, acquired := s.tryAcquirePreview(parent)
	if !acquired {
		return true, nil
	}
	defer release()
	allowed := map[string]bool{}
	for _, request := range requests {
		allowed[request.Key] = true
	}
	published := map[string]bool{}
	err := provider.StreamThumbnails(ctx, requests, func(key string, data []byte) {
		if ctx.Err() != nil || !allowed[key] || published[key] || !validPreview(data) {
			return
		}
		published[key] = true
		publish(key, data)
		// Publish first; a slow or failing cache write cannot hold the image back.
		if os.MkdirAll(s.PreviewDirectory, 0700) == nil {
			temp, err := os.CreateTemp(s.PreviewDirectory, ".preview-*")
			if err == nil {
				path := temp.Name()
				_, writeErr := temp.Write(data)
				closeErr := temp.Close()
				if writeErr == nil && closeErr == nil {
					_ = os.Rename(path, paths[key])
				}
				_ = os.Remove(path)
			}
		}
	})
	s.trimPreviewCache()
	// A transfer preempted this low-priority request; the visible queue may retry.
	if ctx.Err() == context.Canceled && parent.Err() == nil {
		return true, nil
	}
	return false, err
}
func (s *Service) trimPreviewCache() {
	entries, err := os.ReadDir(s.PreviewDirectory)
	if err != nil {
		return
	}
	type saved struct {
		path     string
		modified time.Time
		size     int64
	}
	files := []saved{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jpg" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, saved{filepath.Join(s.PreviewDirectory, entry.Name()), info.ModTime(), info.Size()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	count := len(files)
	for _, file := range files {
		if count <= 256 && total <= 64*1024*1024 {
			break
		}
		if os.Remove(file.path) == nil {
			count--
			total -= file.size
		}
	}
}

// tryAcquirePreview owns the low-priority CLI slot until release.
func (s *Service) tryAcquirePreview(parent context.Context) (context.Context, func(), bool) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	slot := s.cliSlot()
	s.previewMu.Lock()
	if s.previewWaiters > 0 {
		s.previewMu.Unlock()
		cancel()
		return nil, nil, false
	}
	select {
	case slot <- struct{}{}:
		s.previewCancel = cancel
		s.previewMu.Unlock()
	default:
		s.previewMu.Unlock()
		cancel()
		return nil, nil, false
	}
	return ctx, func() {
		cancel()
		s.previewMu.Lock()
		s.previewCancel = nil
		<-slot
		s.previewMu.Unlock()
	}, true
}
