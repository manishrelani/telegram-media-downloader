package tdl

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const previewExtensionName = "tmm-previews-v3"

func (a DomainAdapter) StreamThumbnails(ctx context.Context, requests []domain.PreviewRequest, emit func(string, []byte)) error {
	if len(requests) == 0 || len(requests) > 12 {
		return fmt.Errorf("invalid visible thumbnail batch")
	}
	allowed := map[string]bool{}
	for _, request := range requests {
		if request.Key == "" || len(request.Key) > 2048 || allowed[request.Key] {
			return fmt.Errorf("invalid thumbnail keys")
		}
		allowed[request.Key] = true
	}
	helper := a.ThumbnailHelper
	if helper == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		name := "tdl-" + previewExtensionName
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		helper = filepath.Join(filepath.Dir(exe), name)
	}
	if info, err := os.Stat(helper); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("thumbnail helper is unavailable")
	}
	executor, err := a.Client.executor()
	if err != nil {
		return err
	}
	if native, ok := executor.(OSExecutor); ok {
		native.PreviewProcessGroup = true
		executor = native
	}
	streamer, ok := executor.(StreamExecutor)
	if !ok {
		return fmt.Errorf("streaming thumbnails unavailable")
	}
	ready, _, err := executor.Run(ctx, previewExtensionName, "--identity")
	if err != nil || strings.TrimSpace(string(ready)) != previewExtensionName {
		if _, _, err = executor.Run(ctx, "extension", "install", helper); err != nil {
			return fmt.Errorf("could not register thumbnail helper")
		}
		ready, _, err = executor.Run(ctx, previewExtensionName, "--identity")
		if err != nil || strings.TrimSpace(string(ready)) != previewExtensionName {
			return fmt.Errorf("thumbnail helper is unavailable")
		}
	}
	dir, err := os.MkdirTemp("", "tmm-preview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.json")
	encoded, err := json.Marshal(requests)
	if err != nil {
		return err
	}
	if err = os.WriteFile(input, encoded, 0600); err != nil {
		return err
	}
	seen := map[string]bool{}
	return streamer.RunStream(ctx, []string{previewExtensionName, "--input", input}, func(stream, line string) {
		if stream != "stdout" || len(line) > 400000 || ctx.Err() != nil {
			return
		}
		var frame struct {
			Protocol string `json:"protocol"`
			Key      string `json:"key"`
			Image    []byte `json:"image"`
		}
		if json.Unmarshal([]byte(line), &frame) != nil || frame.Protocol != previewExtensionName || !allowed[frame.Key] || seen[frame.Key] || !validNetworkThumbnail(frame.Image) {
			return
		}
		seen[frame.Key] = true
		emit(frame.Key, frame.Image)
	})
}

func validNetworkThumbnail(data []byte) bool {
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
