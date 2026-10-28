package tdl

import (
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"strings"
)

func thumbnailReference(raw json.RawMessage) *domain.ThumbnailReference {
	var message struct {
		Media struct {
			Spoiler  bool
			Document struct {
				ID            int64
				AccessHash    int64
				FileReference []byte
				DCID          int
				Thumbs        []struct {
					Type  string
					W, H  int
					Size  int64
					Sizes []int64
				}
			}
		}
	}
	if json.Unmarshal(raw, &message) != nil || message.Media.Spoiler {
		return nil
	}
	d := message.Media.Document
	if d.ID == 0 || d.AccessHash == 0 || len(d.FileReference) == 0 || len(d.FileReference) > 4096 || d.DCID < 1 || d.DCID > 10 {
		return nil
	}
	var chosen *domain.ThumbnailReference
	for _, thumb := range d.Thumbs {
		if !strings.Contains("smxywabcd", thumb.Type) || len(thumb.Type) != 1 || thumb.W <= 0 || thumb.H <= 0 || thumb.W > 1280 || thumb.H > 1280 {
			continue
		}
		size := thumb.Size
		for _, value := range thumb.Sizes {
			if value > size {
				size = value
			}
		}
		if size <= 0 || size > 256*1024 {
			continue
		}
		candidate := &domain.ThumbnailReference{ID: d.ID, AccessHash: d.AccessHash, FileReference: d.FileReference, DCID: d.DCID, ThumbSize: thumb.Type, Size: size, Width: thumb.W, Height: thumb.H}
		// Prefer the smallest image that covers a 96px row at 2×, otherwise the largest available.
		area := thumb.W * thumb.H
		ready := thumb.W >= 192 && thumb.H >= 108
		if chosen == nil {
			chosen = candidate
			continue
		}
		previousReady := chosen.Width >= 192 && chosen.Height >= 108
		if (ready && !previousReady) || (ready && previousReady && area < chosen.Width*chosen.Height) || (!ready && !previousReady && area > chosen.Width*chosen.Height) {
			chosen = candidate
		}
	}
	return chosen
}
