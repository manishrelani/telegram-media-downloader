// A thumbnail-only tdl extension. No full-document download path is accepted.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"image/jpeg"
	"os"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/iyear/tdl/extension"
	"go.uber.org/zap"
)

const maxThumbnailBytes = 256 * 1024
const identity = "tmm-previews-v3"

type Reference struct {
	ID            int64  `json:"id"`
	AccessHash    int64  `json:"accessHash"`
	FileReference []byte `json:"fileReference"`
	DCID          int    `json:"dcId"`
	ThumbSize     string `json:"thumbSize"`
	Size          int64  `json:"size"`
}
type fileAPI interface {
	UploadGetFile(context.Context, *tg.UploadGetFileRequest) (tg.UploadFileClass, error)
}

func fetch(ctx context.Context, api fileAPI, ref Reference) ([]byte, error) {
	allowed := map[string]bool{"s": true, "m": true, "x": true, "y": true, "w": true, "a": true, "b": true, "c": true, "d": true}
	if !allowed[ref.ThumbSize] || ref.ID == 0 || ref.AccessHash == 0 || len(ref.FileReference) == 0 || len(ref.FileReference) > 4096 || ref.DCID < 1 || ref.DCID > 10 || ref.Size <= 0 || ref.Size > maxThumbnailBytes {
		return nil, errors.New("invalid thumbnail location")
	}
	result, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{Location: &tg.InputDocumentFileLocation{ID: ref.ID, AccessHash: ref.AccessHash, FileReference: ref.FileReference, ThumbSize: ref.ThumbSize}, Offset: 0, Limit: maxThumbnailBytes})
	if err != nil {
		return nil, errors.New("thumbnail unavailable")
	}
	file, ok := result.(*tg.UploadFile)
	if !ok || len(file.Bytes) > maxThumbnailBytes {
		return nil, errors.New("invalid thumbnail response")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(file.Bytes))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1280 || config.Height > 1280 {
		return nil, errors.New("invalid thumbnail image")
	}
	if _, err := jpeg.Decode(bytes.NewReader(file.Bytes)); err != nil {
		return nil, errors.New("invalid thumbnail image")
	}
	return file.Bytes, nil
}
func main() {
	input := flag.String("input", "", "private thumbnail request file")
	version := flag.Bool("identity", false, "print helper identity without network access")
	flag.Parse()
	if *version {
		os.Stdout.WriteString(identity + "\n")
		return
	}
	if *input == "" {
		return
	}
	data, err := os.ReadFile(*input)
	if err != nil || len(data) > 1024*1024 {
		return
	}
	var refs []Request
	if json.Unmarshal(data, &refs) != nil || len(refs) > 12 || len(refs) == 0 {
		return
	}
	// The host owns the session handoff; this extension never asks for a login.
	// A hard deadline also bounds authentication/reconnection before Handler starts.
	timer := time.AfterFunc(14*time.Second, func() { os.Exit(0) })
	defer timer.Stop()
	extension.New(extension.Options{Logger: zap.NewNop(), Middlewares: []telegram.Middleware{}})(func(parent context.Context, e *extension.Extension) error {
		ctx, cancel := context.WithTimeout(parent, 11*time.Second)
		defer cancel()

		type connection struct {
			once sync.Once
			pool telegram.CloseInvoker
			api  fileAPI
			err  error
		}
		pools := map[int]*connection{}
		var poolMu sync.Mutex
		resolve := func(ctx context.Context, dc int) (fileAPI, error) {
			if dc < 1 || dc > 10 {
				return nil, errors.New("invalid thumbnail datacenter")
			}
			poolMu.Lock()
			state := pools[dc]
			if state == nil {
				state = &connection{}
				pools[dc] = state
			}
			poolMu.Unlock()
			state.once.Do(func() {
				state.pool, state.err = e.Client().DC(ctx, dc, 2)
				if state.err == nil {
					state.api = tg.NewClient(state.pool)
				}
			})
			return state.api, state.err
		}
		encoder := json.NewEncoder(os.Stdout)
		_ = streamBatch(ctx, refs, resolve, func(key string, image []byte) {
			_ = encoder.Encode(struct {
				Protocol string `json:"protocol"`
				Key      string `json:"key"`
				Image    []byte `json:"image"`
			}{identity, key, image})
		})
		for _, state := range pools {
			if state.pool != nil {
				_ = state.pool.Close()
			}
		}

		return nil
	})
}
