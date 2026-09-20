package team

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// imagegenArtistRoot returns the on-disk directory that holds Artist's
// generated images. Mirrors imagegen.outputDir() but lives in the team
// package so we don't expose an internal helper from internal/imagegen.
// Override with HIVEX_IMAGEGEN_DIR; default ~/.hivex/office/artist.
func imagegenArtistRoot() string {
	if root := strings.TrimSpace(os.Getenv("HIVEX_IMAGEGEN_DIR")); root != "" {
		return root
	}
	if home := config.RuntimeHomeDir(); home != "" {
		return filepath.Join(home, ".hivex", "office", "artist")
	}
	return filepath.Join(".hivex", "office", "artist")
}
