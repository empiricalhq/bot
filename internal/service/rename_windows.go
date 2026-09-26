package service

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// renameNoReplace moves oldpath to newpath atomically and fails with an error matching os.ErrExist if newpath exists:
// without MOVEFILE_REPLACE_EXISTING, MoveFileEx refuses an existing target.
func renameNoReplace(oldpath, newpath string) error {
	from, err := windows.UTF16PtrFromString(oldpath)
	if err != nil {
		return fmt.Errorf("rename without replace: %w", err)
	}

	to, err := windows.UTF16PtrFromString(newpath)
	if err != nil {
		return fmt.Errorf("rename without replace: %w", err)
	}

	err = windows.MoveFileEx(from, to, 0)
	if err != nil {
		return fmt.Errorf("rename without replace: %w", err)
	}

	return nil
}
