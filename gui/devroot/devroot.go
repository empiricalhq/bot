//go:build !dev

// Package devroot makes `wails dev` run from the repo root when imported for
// its side effect. Without the dev build tag it does nothing, so built apps
// keep resolving paths against their own working directory.
package devroot
