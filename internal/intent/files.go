package intent

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/files"
)

var (
	imageExts   = []string{"png", "jpg", "jpeg", "webp", "gif", "bmp", "svg", "heic", "avif"}
	videoExts   = []string{"mp4", "mkv", "webm", "mov", "avi"}
	audioExts   = []string{"mp3", "flac", "ogg", "opus", "wav", "m4a"}
	docExts     = []string{"pdf", "doc", "docx", "odt", "rtf", "txt", "md"}
	sheetExts   = []string{"xlsx", "xls", "ods", "csv"}
	slideExts   = []string{"pptx", "ppt", "odp", "key"}
	archiveExts = []string{"zip", "tar", "gz", "xz", "zst", "7z", "rar"}
	bookExts    = []string{"epub", "mobi", "azw3", "pdf"}
	codeExts    = []string{"go", "py", "c", "h", "rs", "js", "ts", "lua", "sh", "zig", "java", "kt"}
	configExts  = []string{"toml", "yaml", "yml", "conf", "ini", "json"}
	fontExts    = []string{"ttf", "otf", "woff", "woff2"}
)

// kinds names a sort of file by the words people use for it. Singular
// words that are as often part of a name ("config", "code", "book", "doc")
// only count in the plural; "nzlauncher config" looks for a name.
var kinds = map[string][]string{
	"pdf": {"pdf"}, "pdfs": {"pdf"},
	"docs": docExts, "documents": docExts,
	"image": imageExts, "images": imageExts, "picture": imageExts, "pictures": imageExts,
	"pic": imageExts, "pics": imageExts, "photo": imageExts, "photos": imageExts,
	"screenshot": imageExts, "screenshots": imageExts,
	"video": videoExts, "videos": videoExts, "movie": videoExts, "movies": videoExts,
	"recordings": videoExts,
	"audio":      audioExts, "song": audioExts, "songs": audioExts, "music": audioExts, "tracks": audioExts,
	"spreadsheet": sheetExts, "spreadsheets": sheetExts, "sheets": sheetExts,
	"presentation": slideExts, "presentations": slideExts, "slides": slideExts, "decks": slideExts,
	"archive": archiveExts, "archives": archiveExts, "zips": archiveExts,
	"ebook": bookExts, "ebooks": bookExts, "books": bookExts,

	"configs": configExts,
	"fonts":   fontExts,
}

// knownExts are extensions a bare word can name ("png", "go files", "mp4s").
var knownExts = func() map[string]bool {
	m := map[string]bool{}
	for _, exts := range [][]string{imageExts, videoExts, audioExts, docExts, sheetExts, slideExts,
		archiveExts, bookExts, codeExts, configExts, fontExts} {
		for _, e := range exts {
			m[e] = true
		}
	}
	return m
}()

// homeDirs are the folders a word names on its own; a word after "in" may
// also name any folder in home.
var homeDirs = map[string]string{
	"downloads": "Downloads", "download": "Downloads", "desktop": "Desktop",
	"screenshots": "Pictures/Screenshots",
}

var prepositions = map[string]bool{"in": true, "under": true, "inside": true, "from": true, "within": true}

// fileFiller are words that only frame a file query.
var fileFiller = map[string]bool{
	"file": true, "files": true, "find": true, "show": true, "list": true, "open": true,
	"from": true, "on": true, "since": true, "than": true, "that": true, "are": true, "is": true,
	"with": true, "named": true, "called": true, "containing": true, "and": true,
	"at": true, "least": true, "most": true, "or": true, "ago": true, "modified": true,
	"changed": true, "made": true, "created": true, "edited": true, "saved": true, "which": true,
	"under": true, "inside": true, "within": true, "this": true, "last": true, "past": true, ">": true, "<": true,
	"folder": true, "folders": true, "directory": true, "dir": true, "next": true, "but": true, "no": true, "not": true,
}

// articles stand between a preposition and the folder it names.
var articles = map[string]bool{"the": true, "my": true, "a": true, "an": true, "our": true, "your": true}

// yearDash splits "2024-2025" for the range reader.
var yearDash = regexp.MustCompile(`\b((?:19|20)\d\d)-((?:19|20)\d\d)\b`)

// FileQuery is a sentence read as a file search.
type FileQuery struct {
	Filter files.Filter
	// Sure is whether the sentence is clearly about files without a "file"
	// prefix: "pdfs", "screenshots today", "invoice docs bigger than 5mb".
	Sure bool
}

// Files reads q as a file search.
func Files(q string, now time.Time) FileQuery {
	q = sizeDash.ReplaceAllString(q, "$1 - $2")
	q = yearDash.ReplaceAllString(q, "$1 - $2")
	ws := Words(strings.NewReplacer(">", " > ", "<", " < ").Replace(q))
	home := os.Getenv("HOME")
	var f files.Filter
	var hasKind, plural, hasFilesWord, hasVerb, hasTime, hasSize, hasDir bool
	exts := map[string]bool{}
	addExts := func(list []string) {
		for _, e := range list {
			if !exts[e] {
				exts[e] = true
				f.Exts = append(f.Exts, e)
			}
		}
	}
	placed := false // a folder after "in", "under", "from"
	addDir := func(dir string) bool {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			f.InDirs = append(f.InDirs, dir)
			hasDir = true
			return true
		}
		return false
	}
	applySpan := func(s Span, mode string) {
		switch mode {
		case "since":
			f.After = s.From
		case "before":
			f.Before = s.From
		default:
			f.After, f.Before = s.From, s.To
		}
		hasTime = true
	}

	timeMode := ""     // "since" or "before": what the next date means
	sizeCmp := ""      // "min" or "max": what the next size means
	ageCmp := ""       // "older" or "newer": what the next duration means
	sizeAsMin := false // the last size read became a minimum by default, "or less" can flip it
	for i := 0; i < len(ws); i++ {
		w := ws[i].Canon
		// the word before, past "the" and "my": "in my projects"
		prev := ""
		for j := i - 1; j >= 0; j-- {
			if c := ws[j].Canon; !articles[c] {
				prev = c
				break
			}
		}

		if kind, lo, hi, span, n := rangeAt(ws, i, now); n > 0 {
			if kind == "size" {
				f.MinSize, f.MaxSize, hasSize = lo, hi, true
			} else {
				f.After, f.Before, hasTime = span.From, span.To, true
			}
			i += n - 1
			continue
		}

		switch w {
		case "since", "after":
			timeMode = "since"
			continue
		case "before", "until":
			timeMode = "before"
			continue
		case "bigger", "larger", "over", "above", "greater", ">", "min", "minimum":
			sizeCmp = "min"
			continue
		case "more":
			// "no more than 5mb" is a ceiling
			if prev == "no" || prev == "not" {
				sizeCmp = "max"
			} else {
				sizeCmp = "min"
			}
			continue
		case "most", "max", "maximum":
			// "at most 5mb"
			sizeCmp = "max"
			continue
		case "least":
			sizeCmp = "min"
			continue
		case "up":
			// "up to 5mb"
			if i+1 < len(ws) && ws[i+1].Canon == "to" {
				sizeCmp = "max"
				i++
				continue
			}
		case "or":
			// "5mb or less" turns the size just read into a ceiling
			if i+1 < len(ws) {
				switch ws[i+1].Canon {
				case "less", "smaller", "under", "lower", "below", "down":
					if sizeAsMin {
						f.MinSize, f.MaxSize = 0, f.MinSize
					}
					i++
					continue
				case "more", "bigger", "larger", "greater", "over", "above":
					i++
					continue
				}
			}
		case "smaller", "under", "below", "less", "<":
			// "under" is also a place: "under projects"
			if w == "under" && i+1 < len(ws) {
				if _, n := Size(ws, i+1, true); n == 0 {
					break
				}
			}
			sizeCmp = "max"
			continue
		case "older":
			ageCmp = "older"
			continue
		case "newer", "younger":
			ageCmp = "newer"
			continue
		case "big", "large":
			if sizeCmp == "" && !nextIsSize(ws, i) {
				f.MinSize, hasSize = 100<<20, true
				continue
			}
		case "latest", "newest", "last":
			// newest first is the order anyway; the word only says "files"
			if w != "last" || i+1 == len(ws) {
				hasTime = true
				continue
			}
		case "huge":
			f.MinSize, hasSize = 1<<30, true
			continue
		case "small", "tiny":
			f.MaxSize, hasSize = 1<<20, true
			continue
		}

		if ageCmp != "" {
			if d, n := Duration(ws, i); n > 0 {
				if ageCmp == "older" {
					f.Before = now.Add(-d)
				} else {
					f.After = now.Add(-d)
				}
				hasTime, ageCmp = true, ""
				i += n - 1
				continue
			}
		}
		if size, n := Size(ws, i, sizeCmp != ""); n > 0 {
			if sizeCmp == "max" {
				f.MaxSize = size
			} else {
				f.MinSize = size
			}
			hasSize, sizeAsMin, sizeCmp = true, sizeCmp == "", ""
			i += n - 1
			continue
		}
		if span, n := When(ws, i, now); n > 0 {
			applySpan(span, timeMode)
			timeMode = ""
			i += n - 1
			continue
		}
		if prepositions[prev] || timeMode != "" {
			if span, ok := Year(w, now); ok {
				applySpan(span, timeMode)
				timeMode = ""
				continue
			}
		}

		exts, ok := kinds[w]
		if !ok {
			exts, ok = kinds[ws[i].Raw] // "audio" is a synonym elsewhere
		}
		if ok {
			// "in documents", "from pictures" name a folder, not a kind
			if dir := homeChild(home, ws[i].Raw); prepositions[prev] && dir != "" && addDir(dir) {
				placed = true
				continue
			}
			hasKind = true
			plural = plural || strings.HasSuffix(w, "s") && w != "music"
			addExts(exts)
			if w == "screenshot" || w == "screenshots" {
				// the folder comes with the word, it is not a second signal
				had := hasDir
				if !addDir(filepath.Join(home, "Pictures/Screenshots")) {
					f.Terms = append(f.Terms, "screenshot")
				}
				hasDir = had
			}
			continue
		}
		if dir, ok := homeDirs[w]; ok && addDir(filepath.Join(home, dir)) {
			placed = placed || prepositions[prev]
			continue
		}
		if prepositions[prev] && w != "" {
			if dir := homeChild(home, ws[i].Raw); dir != "" && addDir(dir) {
				placed = true
				continue
			}
		}
		if strings.HasPrefix(ws[i].Raw, "~/") || strings.HasPrefix(ws[i].Raw, "/") {
			if addDir(filepath.Clean(strings.Replace(ws[i].Raw, "~", home, 1))) {
				placed = true
				continue
			}
		}
		// "pngs", "mp4s"
		if stem, ok := strings.CutSuffix(w, "s"); ok && knownExts[stem] && len(stem) >= 2 {
			addExts([]string{stem})
			hasKind, plural = true, true
			continue
		}
		// ".go", "go files"
		if stem, ok := strings.CutPrefix(w, "."); ok && knownExts[stem] {
			addExts([]string{stem})
			hasKind = true
			continue
		}
		if knownExts[w] && i+1 < len(ws) && (ws[i+1].Canon == "files" || ws[i+1].Canon == "file") {
			addExts([]string{w})
			hasKind = true
			continue
		}

		switch {
		case w == "file" || w == "files":
			hasFilesWord = true
		case w == "find" || w == "show" || w == "list":
			hasVerb = true
		}
		if ws[i].Filler || fileFiller[w] || ws[i].Raw == "" {
			continue
		}
		f.Terms = append(f.Terms, strings.ToLower(ws[i].Raw))
	}

	signals := 0
	for _, s := range []bool{hasKind, hasFilesWord, hasVerb, hasTime, hasSize, hasDir} {
		if s {
			signals++
		}
	}
	// a kind on its own is not enough unless plural: "screenshot" is the
	// capture action, "screenshots" is the folder
	sure := plural && hasKind || signals >= 2 && (hasKind || hasFilesWord || hasDir || hasTime && hasSize) ||
		placed && len(f.Terms) > 0 // "invoice in documents"
	return FileQuery{Filter: f, Sure: sure}
}

// sizeDash splits "1-5mb" and "1mb-5mb" for the range reader; a date like
// 2026-10-01 has no unit after its digits and stays whole.
var sizeDash = regexp.MustCompile(`(\d(?:[a-zA-Z]+)?)-(\d+(?:\.\d+)?[a-zA-Z]+)\b`)

// rangeSeps join the two ends of a range; "and" only after "between".
func rangeSep(w string, between bool) bool {
	switch w {
	case "to", "-", "until", "till", "through", "thru":
		return true
	case "and":
		return between
	}
	return false
}

// rangeAt reads a range at ws[i:]: sizes ("between 1mb and 5mb", "1 to 5 mb",
// "1-5mb") or dates ("from june to august", "between march and may 2025",
// "2024 to 2025"). kind is "size" or "date"; n is 0 when there is none.
func rangeAt(ws []Word, i int, now time.Time) (kind string, lo, hi int64, span Span, n int) {
	j, between := i, false
	if c := ws[j].Canon; c == "between" || c == "from" {
		between, j = c == "between", j+1
	}
	if j >= len(ws) {
		return "", 0, 0, Span{}, 0
	}

	// sizes; the left end may borrow the right end's unit: "1 to 5 mb"
	left, ln := Size(ws, j, true)
	bare := -1.0
	if ln == 0 {
		if v, err := strconv.ParseFloat(ws[j].Canon, 64); err == nil {
			bare, ln = v, 1
		}
	}
	if ln > 0 && j+ln < len(ws) && rangeSep(ws[j+ln].Canon, between) {
		k := j + ln + 1
		if right, rn := Size(ws, k, true); rn > 0 {
			if bare >= 0 {
				left = int64(bare * float64(sizeUnitAt(ws, k)))
			}
			return "size", left, right, Span{}, k + rn - i
		}
	}

	// dates
	from, ln := dateAt(ws, j, now)
	if ln == 0 || j+ln >= len(ws) || !rangeSep(ws[j+ln].Canon, between) {
		return "", 0, 0, Span{}, 0
	}
	k := j + ln + 1
	to, rn := dateAt(ws, k, now)
	if rn == 0 {
		return "", 0, 0, Span{}, 0
	}
	// "between march and may 2025": the year said once is both ends'
	if m, isMonth := monthOf(ws[j].Canon); isMonth && ln == 1 && rn == 2 {
		y := to.From.Year()
		if m > to.From.Month() {
			y--
		}
		from.From = time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	}
	// each end picked its last occurrence; "from september to november" in
	// October means last year's September, since November has not come
	if from.From.After(to.From) {
		from.From = from.From.AddDate(-1, 0, 0)
	}
	// an open right end ("to today") runs up to now
	return "date", 0, 0, Span{from.From, to.To}, k + rn - i
}

// dateAt is a moment or period at ws[i:], a bare year included.
func dateAt(ws []Word, i int, now time.Time) (Span, int) {
	if i >= len(ws) {
		return Span{}, 0
	}
	if span, n := When(ws, i, now); n > 0 {
		return span, n
	}
	if ws[i].Canon == "now" {
		return Span{From: now}, 1 // an open end: up to now
	}
	if span, ok := Year(ws[i].Canon, now); ok {
		return span, 1
	}
	return Span{}, 0
}

// sizeUnitAt is the unit of the size at ws[k:], in bytes.
func sizeUnitAt(ws []Word, k int) int64 {
	_, unit, _ := splitNumber(ws[k].Canon)
	if unit == "" && k+1 < len(ws) {
		unit = ws[k+1].Canon
	}
	if unit == "m" {
		return 1 << 20
	}
	if n, ok := sizeUnits[unit]; ok {
		return n
	}
	return 1
}

func nextIsSize(ws []Word, i int) bool {
	for j := i + 1; j < len(ws) && j <= i+2; j++ {
		if _, n := Size(ws, j, true); n > 0 {
			return true
		}
	}
	return false
}

// homeChild finds a folder in home by name, ignoring case: "in projects".
func homeChild(home, name string) string {
	entries, err := os.ReadDir(home)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(home, e.Name())
		}
	}
	return ""
}
