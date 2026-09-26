package service

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// renameNoReplace moves oldpath to newpath atomically and fails with an error matching os.ErrExist if newpath exists.
func renameNoReplace(oldpath, newpath string) error {
	err := unix.RenamexNp(oldpath, newpath, unix.RENAME_EXCL)
	if err != nil {
		return fmt.Errorf("rename without replace: %w", err)
	}

	return nil
}
