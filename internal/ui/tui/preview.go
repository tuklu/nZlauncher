package tui

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // DecodeConfig learns these formats
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

// The preview pane opens beside the list when the terminal is this wide and
// the selected result has a file to show.
const (
	previewMinWidth = 80
	previewBytes    = 4096
)

// preview is the selected result's file, read once per path.
type preview struct {
	path  string
	lines []string // text, or a description for what cannot be shown as text
	// an image the terminal can draw: pixel size, and a PNG of it
	imgW, imgH int
	png        string
	sent       string // what the terminal has: path and cell size
	id         int    // the image id it has it under
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
		if kittyGraphics() {
			p.imgW, p.imgH = cfg.Width, cfg.Height
			p.png = asPNG(path, format)
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

// sized is set once the terminal has said how big it is: an image sent
// at the guessed size before that would be sent again a frame later.
var sized bool

// render fits the preview into width by height, border included.
func (p *preview) render(st tuiStyles, width, height int) string {
	inner := max(width-4, 1)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(st.border.GetForeground()).
		Padding(0, 1).
		Width(width - 2).
		Height(max(height-2, 1))
	if p.png != "" && p.imgW > 0 && p.imgH > 0 && sized {
		// not styled: the placeholders' colour is the image id
		return box.Render(p.imageCells(inner, height-2))
	}
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
	return box.Render(st.badge.Render(strings.Join(out, "\n")))
}

// Images go through the kitty graphics protocol as Unicode placeholders:
// the image is sent once as a virtual placement, and the pane draws it with
// ordinary (if odd) characters, so the line-by-line redraw of the TUI
// cannot tear it. Ghostty and kitty support that; elsewhere the pane keeps
// its text description.

// Each send gets a fresh image id and deletes the last one: Ghostty keeps
// the first placement's size for an id that is sent again. An id travels in
// the placeholders' 256-colour foreground, so ids cycle below 256.
const (
	firstImageID = 101
	lastImageID  = 250
)

var nextImageID = firstImageID

func newImageID() int {
	id := nextImageID
	if nextImageID++; nextImageID > lastImageID {
		nextImageID = firstImageID
	}
	return id
}

// shownID is the image on screen, for clearing it when the TUI closes.
var shownID int

// kittyGraphics reports whether this terminal draws Unicode placeholders.
func kittyGraphics() bool {
	if os.Getenv("NZ_NO_IMAGES") != "" {
		return false
	}
	return os.Getenv("TERM_PROGRAM") == "ghostty" || os.Getenv("TERM") == "xterm-ghostty" ||
		os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != ""
}

// asPNG is a PNG of the image: the file itself, or a converted copy in the
// cache, since the protocol takes PNG and nothing else. "" when it fails.
func asPNG(path, format string) string {
	if format == "png" {
		return path
	}
	sum := sha1.Sum([]byte(path))
	dir := filepath.Join(cacheDir(), "tui-preview")
	out := filepath.Join(dir, hex.EncodeToString(sum[:8])+".png")
	src, err := os.Stat(path)
	if done, err2 := os.Stat(out); err == nil && err2 == nil && !done.ModTime().Before(src.ModTime()) {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil || os.MkdirAll(dir, 0o700) != nil {
		return ""
	}
	tmp, err := os.CreateTemp(dir, "*.part")
	if err != nil {
		return ""
	}
	if png.Encode(tmp, img) != nil || tmp.Close() != nil || os.Rename(tmp.Name(), out) != nil {
		os.Remove(tmp.Name())
		return ""
	}
	return out
}

func cacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "nzlauncher")
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "nzlauncher")
}

// rowDiacritics are the first entries of kitty's row and column diacritics:
// the nth one on a placeholder means row n.
var rowDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A,
	0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365,
	0x0366, 0x0367, 0x0368, 0x0369, 0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F,
	0x0483, 0x0484, 0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
}

const placeholder = '\U0010EEEE'

// cellAspect is a cell's height over its width, from the pixel size the
// terminal reports; 2 when it reports none.
func cellAspect() float64 {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Xpixel == 0 || ws.Ypixel == 0 || ws.Col == 0 || ws.Row == 0 {
		return 2
	}
	return (float64(ws.Ypixel) / float64(ws.Row)) / (float64(ws.Xpixel) / float64(ws.Col))
}

// imageCells fits the image into cols by rows at its own aspect and returns
// the placeholder lines. The first time an image or size is drawn, the
// transmission rides along.
func (p *preview) imageCells(cols, rows int) string {
	rows = min(rows, len(rowDiacritics))
	aspect := cellAspect()
	c := cols
	r := int(float64(cols) * float64(p.imgH) / float64(p.imgW) / aspect)
	if r > rows {
		r = rows
		c = int(float64(rows) * aspect * float64(p.imgW) / float64(p.imgH))
	}
	c, r = max(min(c, cols), 1), max(r, 1)

	var b strings.Builder
	if key := fmt.Sprintf("%s %dx%d", p.png, c, r); p.sent != key {
		if shownID != 0 {
			fmt.Fprintf(&b, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", shownID)
		}
		p.id = newImageID()
		fmt.Fprintf(&b, "\x1b_Ga=T,U=1,f=100,t=f,i=%d,c=%d,r=%d,q=2;%s\x1b\\",
			p.id, c, r, base64.StdEncoding.EncodeToString([]byte(p.png)))
		p.sent, shownID = key, p.id
	}
	for row := range r {
		if row > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "\x1b[38;5;%dm", p.id)
		// the first cell names its row and column; the rest follow on
		b.WriteRune(placeholder)
		b.WriteRune(rowDiacritics[row])
		b.WriteRune(rowDiacritics[0])
		b.WriteString(strings.Repeat(string(placeholder), c-1))
		b.WriteString("\x1b[39m")
	}
	return b.String()
}

// clearImage removes the pane's image from the terminal.
func clearImage() string {
	if shownID == 0 {
		return ""
	}
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", shownID)
}
