package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // DecodeConfig learns these formats
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The preview pane opens beside the list when the terminal is this wide and
// the selected result has a file to show.
const (
	previewMinWidth = 100
	previewBytes    = 4096
)

// preview is the selected result's file, read once per path.
type preview struct {
	path  string
	lines []string // text, or a description for what cannot be shown as text
}

func loadPreview(path string) preview {
	p := preview{path: path}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	f, err := os.Open(path)
	if err != nil {
		p.lines = []string{"cannot read it"}
		return p
	}
	defer f.Close()

	if cfg, format, err := image.DecodeConfig(f); err == nil {
		info, _ := os.Stat(path)
		p.lines = []string{fmt.Sprintf("image · %s · %d×%d", format, cfg.Width, cfg.Height)}
		if info != nil {
			p.lines = append(p.lines, fmt.Sprintf("%.1f KiB", float64(info.Size())/1024))
		}
		return p
	}
	switch ext {
	case "webp", "svg", "bmp", "avif", "heic":
		p.lines = []string{"image · " + ext}
		return p
	}

	f.Seek(0, 0)
	buf := make([]byte, previewBytes)
	n, _ := f.Read(buf)
	buf = buf[:n]
	if bytes.IndexByte(buf, 0) >= 0 {
		p.lines = []string{"binary file"}
		return p
	}
	text := strings.ReplaceAll(strings.ToValidUTF8(string(buf), "?"), "\t", "    ")
	p.lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	return p
}

// render fits the preview into width by height, border included.
func (p preview) render(st tuiStyles, width, height int) string {
	inner := max(width-4, 1)
	var out []string
	for _, line := range p.lines {
		if len(out) == height-2 {
			break
		}
		r := []rune(line)
		for lipgloss.Width(string(r)) > inner && len(r) > 0 {
			r = r[:len(r)-1]
		}
		out = append(out, string(r))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(st.border.GetForeground()).
		Padding(0, 1).
		Width(width - 2).
		Height(max(height-2, 1)).
		Render(st.badge.Render(strings.Join(out, "\n")))
}
