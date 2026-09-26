package service

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// renameNoReplace moves oldpath to newpath atomically and fails with an error matching os.ErrExist if newpath exists.
// A filesystem without RENAME_NOREPLACE answers EINVAL and a kernel before 3.15 answers ENOSYS; both are reported as
// errors.ErrUnsupported.
func renameNoReplace(oldpath, newpath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldpath, unix.AT_FDCWD, newpath, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
		return fmt.Errorf("rename without replace: %w: %w", errors.ErrUnsupported, err)
	}

	if err != nil {
		return fmt.Errorf("rename without replace: %w", err)
	}

	return nil
}
