package server

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

// faviconSize is the width and height clients require of the server icon.
const faviconSize = 64

// loadFavicon reads the server icon like vanilla: server-icon.png in the
// working directory, or else the world's icon.png. It returns nil when
// neither exists.
func loadFavicon(worldDir string) ([]byte, error) {
	for _, path := range []string{"server-icon.png", filepath.Join(worldDir, "icon.png")} {
		data, err := os.ReadFile(path)

		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			return nil, err
		}

		img, err := png.DecodeConfig(bytes.NewReader(data))

		if err != nil {
			return nil, fmt.Errorf("server: %s is not a PNG image: %w", path, err)
		}

		if img.Width != faviconSize || img.Height != faviconSize {
			return nil, fmt.Errorf("server: %s must be %dx%d pixels, not %dx%d", path, faviconSize, faviconSize, img.Width, img.Height)
		}

		return data, nil
	}

	return nil, nil
}
