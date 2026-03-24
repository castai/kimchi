package update

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/progress"
)

type progressWriter struct {
	w           io.Writer
	bar         progress.Model
	total       int64
	written     int64
	lastPercent int
}

func newProgressWriter(w io.Writer, total int64) *progressWriter {
	bar := progress.New(progress.WithDefaultGradient())
	return &progressWriter{w: w, bar: bar, total: total}
}

// Write tracks download progress. Rendering errors are intentionally
// ignored so they never block the actual file download.
func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	if pw.total > 0 {
		pct := int(float64(pw.written) / float64(pw.total) * 100)
		if pct != pw.lastPercent {
			pw.lastPercent = pct
			_, _ = fmt.Fprintf(pw.w, "\r  %s", pw.bar.ViewAs(float64(pw.written)/float64(pw.total)))
		}
	}
	return n, nil
}

func (pw *progressWriter) finish() {
	_, _ = fmt.Fprintln(pw.w)
}
