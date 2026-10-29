package intent

import (
	"os"
	"path/filepath"
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

// kinds names a sort of file by the words people use for it.
var kinds = map[string][]string{
	"pdf": {"pdf"}, "pdfs": {"pdf"},
	"doc": docExts, "docs": docExts, "document": docExts, "documents": docExts,
	"image": imageExts, "images": imageExts, "picture": imageExts, "pictures": imageExts,
	"pic": imageExts, "pics": imageExts, "photo": imageExts, "photos": imageExts,
	"screenshot": imageExts, "screenshots": imageExts,
	"video": videoExts, "videos": videoExts, "movie": videoExts, "movies": videoExts,
	"recording": videoExts, "recordings": videoExts,
	"audio": audioExts, "song": audioExts, "songs": audioExts, "music": audioExts, "track": audioExts, "tracks": audioExts,
	"spreadsheet": sheetExts, "spreadsheets": sheetExts, "sheet": sheetExts, "sheets": sheetExts,
	"presentation": slideExts, "presentations": slideExts, "slides": slideExts, "deck": slideExts, "decks": slideExts,
	"archive": archiveExts, "archives": archiveExts, "zips": archiveExts,
	"ebook": bookExts, "ebooks": bookExts, "book": bookExts, "books": bookExts,
	"code": codeExts, "source": codeExts,
	"config": configExts, "configs": configExts,
	"font": fontExts, "fonts": fontExts,
}

// knownExts are extensions a bare word can name ("png", "go files", "mp4s").
var knownExts = func() map[string]bool {
	m := map[string]bool{}
	for _, exts := range kinds {
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
}

// FileQuery is a sentence read as a file search.
type FileQuery struct {
	Filter files.Filter
	// Sure is whether the sentence is clearly about files without a "file"
	// prefix: "pdfs", "screenshots today", "invoice docs bigger than 5mb".
	Sure bool
}

// Files reads q as a file search.
func Files(q string, now time.Time) FileQuery {
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

	timeMode := "" // "since" or "before": what the next date means
	sizeCmp := ""  // "min" or "max": what the next size means
	ageCmp := ""   // "older" or "newer": what the next duration means
	for i := 0; i < len(ws); i++ {
		w := ws[i].Canon
		prev := ""
		if i > 0 {
			prev = ws[i-1].Canon
		}

		switch w {
		case "since", "after":
			timeMode = "since"
			continue
		case "before", "until":
			timeMode = "before"
			continue
		case "bigger", "larger", "over", "above", "greater", "more", ">":
			sizeCmp = "min"
			continue
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
			hasSize, sizeCmp = true, ""
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
