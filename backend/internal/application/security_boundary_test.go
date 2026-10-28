package application

import (
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityRejectsSymlinkCategory(t *testing.T) {
	s, _ := identityService(t)
	base := t.TempDir()
	chosen := filepath.Join(base, "chosen")
	outside := filepath.Join(base, "outside")
	chat := domain.Chat{ID: 123, Name: "Course"}
	chatroot := filepath.Join(chosen, "Course [123]")
	if err := os.MkdirAll(chatroot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(chatroot, "Videos")); err != nil {
		t.Fatal(err)
	}
	_, err := runIdentityMedia(s, "audit-symlink", chosen, chat)
	if err == nil {
		t.Fatal("symlinked category allowed a download outside destination")
	}
	if _, err := os.Stat(filepath.Join(outside, "Original caption.mp4")); !os.IsNotExist(err) {
		t.Fatal("escaped output was created")
	}

}
func TestSecurityRejectsManifestSavedNameTraversal(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	first, err := runIdentityMedia(s, "audit-first", root, chat)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside", "harmless.command")
	if err := os.Mkdir(filepath.Dir(outside), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(first.Job.Destination, ".telegram-media-manager-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m domain.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for key, entry := range m.Items {
		entry.SavedName = "../../outside/harmless.command"
		entry.SavedPath = ""
		m.Items[key] = entry
	}
	data, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
	_, err = runIdentityMedia(fresh, "audit-second", root, chat)
	if err == nil {
		t.Fatal("forged manifest accepted an outside file")
	}
	if g.downloadCalls != 1 {
		t.Fatal("unsafe manifest started another transfer")
	}

}
func TestSecurityPrivateWorkIgnoresSharedFolderLinks(t *testing.T) {
	s, _ := identityService(t)
	chosen := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	chatroot := filepath.Join(chosen, "Course [123]")
	outside := t.TempDir()
	os.MkdirAll(chatroot, 0700)
	os.Symlink(outside, filepath.Join(chatroot, ".tdl-work"))
	result, err := runIdentityMedia(s, "private-work", chosen, chat)
	if err != nil {
		t.Fatalf("shared work-directory entry affected private work: %v", err)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("working data escaped into shared symlink")
	}
	if _, err := os.Stat(filepath.Join(mediaWorkDir(result.Job), "export.json")); err != nil {
		t.Fatal("private recovery export missing")
	}
	if relative, err := filepath.Rel(chosen, mediaWorkDir(result.Job)); err == nil && filepath.IsLocal(relative) {
		t.Fatal("working data still lives under shared destination")
	}
}
