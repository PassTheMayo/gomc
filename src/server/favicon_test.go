package server

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// writePNG writes a blank PNG of the given size.
func writePNG(t *testing.T, path string, size int) []byte {
	t.Helper()

	var buf bytes.Buffer

	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func TestLoadFavicon(t *testing.T) {
	t.Chdir(t.TempDir())

	worldDir := "world"

	if err := os.Mkdir(worldDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if icon, err := loadFavicon(worldDir); icon != nil || err != nil {
		t.Fatalf("without an icon loadFavicon returned %d bytes, %v", len(icon), err)
	}

	// The world's icon is used when the server has none of its own.
	worldIcon := writePNG(t, filepath.Join(worldDir, "icon.png"), faviconSize)

	if icon, err := loadFavicon(worldDir); !bytes.Equal(icon, worldIcon) || err != nil {
		t.Fatalf("the world icon wasn't loaded: %v", err)
	}

	serverIcon := writePNG(t, "server-icon.png", faviconSize)

	if icon, err := loadFavicon(worldDir); !bytes.Equal(icon, serverIcon) || err != nil {
		t.Fatalf("server-icon.png wasn't preferred: %v", err)
	}

	writePNG(t, "server-icon.png", 32)

	if _, err := loadFavicon(worldDir); err == nil {
		t.Fatal("a 32x32 icon was accepted")
	}
}
