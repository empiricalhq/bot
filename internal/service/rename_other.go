//go:build !linux && !darwin && !windows

package service

import (
	"errors"
	"fmt"
)

func renameNoReplace(_, _ string) error {
	return fmt.Errorf("rename without replace: %w", errors.ErrUnsupported)
}
