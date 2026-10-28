package main

import (
	"bytes"
	"context"
	"github.com/gotd/td/tg"
	"image"
	"image/jpeg"
	"testing"
)

type fakeAPI struct {
	calls    int
	data     []byte
	location *tg.InputDocumentFileLocation
}

func (f *fakeAPI) UploadGetFile(_ context.Context, r *tg.UploadGetFileRequest) (tg.UploadFileClass, error) {
	f.calls++
	f.location = r.Location.(*tg.InputDocumentFileLocation)
	return &tg.UploadFile{Bytes: f.data}, nil
}
func TestOnlyThumbnailLocationsAreAccepted(t *testing.T) {
	for _, kind := range []string{"", "i", "j", "v", "video", "../"} {
		api := &fakeAPI{}
		_, err := fetch(context.Background(), api, Reference{ID: 1, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: kind, Size: 100})
		if err == nil || api.calls != 0 {
			t.Fatalf("unsafe type %q invoked API", kind)
		}
	}
}
func TestThumbnailDownloadUsesBoundedFileRequest(t *testing.T) {
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 320, 180)), nil); err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{data: data.Bytes()}
	got, err := fetch(context.Background(), api, Reference{ID: 1, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: "m", Size: int64(data.Len())})
	if err != nil || len(got) == 0 || api.location.ThumbSize != "m" || api.calls != 1 {
		t.Fatalf("fetch: %v, calls=%d", err, api.calls)
	}
}
func TestThumbnailSizeAndDimensionsAreBounded(t *testing.T) {
	api := &fakeAPI{}
	_, err := fetch(context.Background(), api, Reference{ID: 1, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: "m", Size: 300000})
	if err == nil || api.calls != 0 {
		t.Fatal("oversized metadata accepted")
	}
	api.data = bytes.Repeat([]byte{1}, 100)
	_, err = fetch(context.Background(), api, Reference{ID: 1, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: "m", Size: 100})
	if err == nil {
		t.Fatal("invalid image accepted")
	}
}
