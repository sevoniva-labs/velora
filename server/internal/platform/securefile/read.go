package securefile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Read opens only the final path component beneath an os.Root. This preserves
// operator-selected config and Secret mount directories while preventing the
// selected file from escaping that directory through a symbolic link.
func Read(path string) ([]byte, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == string(filepath.Separator) {
		return nil, errors.New("file path is empty")
	}
	dir, name := filepath.Split(clean)
	if dir == "" {
		dir = "."
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(name)
}
