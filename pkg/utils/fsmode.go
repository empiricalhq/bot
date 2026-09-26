package utils

import "os"

// Permissions for what the bot writes to disk: directories others may enter, and files others may read.
const (
	DirMode  os.FileMode = 0o755
	FileMode os.FileMode = 0o644
)
