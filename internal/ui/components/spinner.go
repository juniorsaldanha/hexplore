package components

import "time"

// spinnerFrames is the classic braille dots cycle used by most CLI
// spinners (cli-spinners' "dots" set).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerFrameInterval = 80 * time.Millisecond

// Spinner returns the animated glyph for instant t — deterministic (the
// same t always yields the same frame) rather than owning a timer of its
// own, so it animates for free off the app's already-continuous render
// loop (the dashboard's fps ticker) as long as the caller passes
// time.Now().
func Spinner(t time.Time) string {
	n := int64(len(spinnerFrames))
	idx := (t.UnixMilli() / spinnerFrameInterval.Milliseconds()) % n
	if idx < 0 {
		idx += n // UnixMilli() is negative before 1970; keep the index in range
	}
	return spinnerFrames[idx]
}
