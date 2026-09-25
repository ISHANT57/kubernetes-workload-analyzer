// Package logging sets up structured logging via the standard library's log/slog (Go 1.21+),
// deliberately not a third-party logging library: slog already gives JSON output, levels and
// structured fields, so an extra dependency would have no justification (AGENTS.md: "No new
// dependency without a stated reason").
package logging

import (
	"log/slog"
	"os"
)

// New returns a JSON structured logger writing to stdout, so it works the same way whether run
// on a laptop or in-cluster (Phase 7), where stdout is what gets collected as pod logs.
//
// Callers must never pass credentials, tokens or kubeconfig contents as log fields (AGENTS.md:
// "the backend never logs config values that could hold credentials") -- this function has no
// way to enforce that; it is a rule every call site must follow.
func New(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
