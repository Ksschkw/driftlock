package updater

import (
	"os"
)

// WriteDoc writes the new content to the file path, overwriting it.
func WriteDoc(filePath, content string) error {
	return os.WriteFile(filePath, []byte(content), 0o644)
}