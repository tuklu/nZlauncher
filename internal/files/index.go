// Package files keeps an in-memory index of the files under the user's
// roots, so a file query is a scan of memory instead of a walk of the disk.
// The daemon builds it in the background, rescans it every few minutes, and
// watches the directories where new files usually land so those appear at
// once. Without the daemon nobody builds it and callers fall back to fd.
package files

import (
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// A full walk of a large home takes seconds of CPU; the watched directories
// cover where new files usually appear in between.
const rescanEvery = 20 * time.Minute

// Entry is one file, 16 bytes: its folder is shared with its siblings and
// its name lives in the packed buffers.
type Entry struct {
	dir   int32  // index into index.dirs
	mtime uint32 // unix seconds; fits until 2106
	size  int64
}

type index struct {
	dirs    []string
	entries []Entry
	built   time.Time
	// every lowercased name, each followed by '\n'; entry i starts at offsets[i]
	names   []byte
	offsets []uint32
	// every name as it is on disk, back to back; entry i starts at origAt[i]
	orig   []byte
	origAt []uint32
	freq   [256]int // how often each byte occurs in names
	// trigram of a lowercased name -> where its list starts in postings and
	// how many entries it holds. A list is the ascending entries containing
	// the trigram, stored as varint gaps: about 1.5 bytes an entry, not 4.
	trigrams map[uint32][2]uint32
	postings []byte
	// extension -> entries with it, so "pdfs" with no name is a lookup
	byExt map[string][]uint32
	// a directory under a dot folder or inside a build or vendor tree
	// ranks below the user's own files
	buried []bool
	// the walk is depth first, so a folder's whole subtree is one run of
	// entries; lastEntry[d] is where the run of dir d's files ends
	firstEntry, lastEntry []uint32
	// dir indexes by path: everything under a folder is one block of it
	sortedDirs []uint32
	// each folder lowercased and relative to its root, for terms that land
	// in the path: "nzlauncher config" finds ~/.config/nzlauncher/config.toml
	lowerDirs []string
	// files the user opens, resolved to entries once per boost list
	boost atomic.Pointer[resolvedBoost]
}

func (ix *index) path(i int) string {
	return filepath.Join(ix.dirs[ix.entries[i].dir], ix.origName(i))
}

// origName views entry i's name as it is on disk, without copying.
func (ix *index) origName(i int) string {
	end := len(ix.orig)
	if i+1 < len(ix.origAt) {
		end = int(ix.origAt[i+1])
	}
	b := ix.orig[ix.origAt[i]:end]
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// list decodes the entries of one trigram.
func (ix *index) list(span [2]uint32) []uint32 {
	out := make([]uint32, 0, span[1])
	buf := ix.postings[span[0]:]
	prev := uint64(0)
	for range span[1] {
		gap, n := binary.Uvarint(buf)
		buf = buf[n:]
		prev += gap
		out = append(out, uint32(prev))
	}
	return out
}

// Config is what to index; Exclude matches a path component ("node_modules")
// or a path below a root ("go/pkg").
type Config struct {
	Roots   []string
	Exclude []string
	Watch   []string
}

var (
	current  atomic.Pointer[index]
	cfg      atomic.Pointer[Config]
	start    sync.Once
	rescanCh = make(chan struct{}, 1)
)

// Ready reports whether a built index can answer queries.
func Ready() bool {
	return current.Load() != nil
}

// Start builds the index in the background and keeps it current. Calling it
// again with a different config rebuilds with the new one.
func Start(c Config) {
	old := cfg.Swap(&c)
	start.Do(func() { go keepCurrent() })
	if old != nil && !slices.Equal(old.Roots, c.Roots) || old != nil && !slices.Equal(old.Exclude, c.Exclude) {
		Rescan()
	}
}

// Rescan asks for a rebuild soon; repeated asks collapse into one.
func Rescan() {
	select {
	case rescanCh <- struct{}{}:
	default:
	}
}

func keepCurrent() {
	go watchHot()
	for {
		c := cfg.Load()
		current.Store(build(*c))
		// a walk allocates far more than the index keeps: hand it back
		// instead of carrying it until the next rescan
		debug.FreeOSMemory()
		select {
		case <-rescanCh:
			// a burst of new files should not cause a rebuild per file
			time.Sleep(time.Second)
		case <-time.After(rescanEvery):
		}
	}
}

func build(c Config) *index {
	ix := &index{built: time.Now(), byExt: map[string][]uint32{}}
	lists := map[uint32][]uint32{}
	dirOf := map[string]int32{}
	for _, root := range c.Roots {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if path != root && excluded(root, path, d.Name(), c.Exclude) {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			dir := filepath.Dir(path)
			idx, ok := dirOf[dir]
			if !ok {
				idx = int32(len(ix.dirs))
				dirOf[dir] = idx
				ix.dirs = append(ix.dirs, dir)
				ix.buried = append(ix.buried, isBuried(strings.TrimPrefix(dir, root)))
				ix.lowerDirs = append(ix.lowerDirs, strings.ToLower(strings.TrimPrefix(dir, root)))
				ix.firstEntry = append(ix.firstEntry, uint32(len(ix.entries)))
				ix.lastEntry = append(ix.lastEntry, 0)
			}
			ix.lastEntry[idx] = uint32(len(ix.entries))
			name := d.Name()
			e := Entry{dir: idx, mtime: uint32(max(info.ModTime().Unix(), 0)), size: info.Size()}
			ix.origAt = append(ix.origAt, uint32(len(ix.orig)))
			ix.orig = append(ix.orig, name...)
			l := strings.ToLower(name)
			ix.offsets = append(ix.offsets, uint32(len(ix.names)))
			ix.names = append(append(ix.names, l...), '\n')
			for k := 0; k < len(l); k++ {
				ix.freq[l[k]]++
			}
			id := uint32(len(ix.entries))
			for k := 0; k+3 <= len(l); k++ {
				t := trigram(l[k:])
				// a name repeating a trigram lists itself once
				if p := lists[t]; len(p) == 0 || p[len(p)-1] != id {
					lists[t] = append(p, id)
				}
			}
			if dot := strings.LastIndexByte(l, '.'); dot > 0 && len(l)-dot <= 6 {
				ext := l[dot+1:]
				ix.byExt[ext] = append(ix.byExt[ext], id)
			}
			ix.entries = append(ix.entries, e)
			return nil
		})
	}
	ix.sortedDirs = make([]uint32, len(ix.dirs))
	for d := range ix.sortedDirs {
		ix.sortedDirs[d] = uint32(d)
	}
	slices.SortFunc(ix.sortedDirs, func(a, b uint32) int { return strings.Compare(ix.dirs[a], ix.dirs[b]) })

	ix.trigrams = make(map[uint32][2]uint32, len(lists))
	total := 0
	for _, p := range lists {
		total += len(p)
	}
	ix.postings = make([]byte, 0, total*2)
	for t, p := range lists {
		ix.trigrams[t] = [2]uint32{uint32(len(ix.postings)), uint32(len(p))}
		prev := uint32(0)
		for _, id := range p {
			ix.postings = binary.AppendUvarint(ix.postings, uint64(id-prev))
			prev = id
		}
		delete(lists, t) // free each list as soon as it is encoded
	}
	ix.postings = slices.Clip(ix.postings)
	return ix
}

// nameBytes is entry i's lowercased name inside the packed buffer.
func (ix *index) nameBytes(i int) []byte {
	end := len(ix.names) - 1
	if i+1 < len(ix.offsets) {
		end = int(ix.offsets[i+1]) - 1
	}
	return ix.names[ix.offsets[i]:end]
}

// lowerName views the packed name as a string without copying it: the
// buffer is never written after the build, which is what makes that safe.
func (ix *index) lowerName(i int) string {
	b := ix.nameBytes(i)
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

func excluded(root, path, name string, exclude []string) bool {
	rel := strings.TrimPrefix(path, root+"/")
	for _, ex := range exclude {
		if name == ex || rel == ex || strings.HasPrefix(rel, ex+"/") {
			return true
		}
	}
	return false
}

// watchHot follows the directories where downloads and screenshots arrive;
// a file created there triggers a rescan instead of waiting for the timer.
func watchHot() {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return
	}
	for _, dir := range cfg.Load().Watch {
		syscall.InotifyAddWatch(fd, dir, syscall.IN_CREATE|syscall.IN_MOVED_TO|syscall.IN_DELETE|syscall.IN_MOVED_FROM|syscall.IN_CLOSE_WRITE)
	}
	buf := make([]byte, 64*1024)
	for {
		if n, err := syscall.Read(fd, buf); err != nil || n <= 0 {
			if err == syscall.EINTR {
				continue
			}
			return
		}
		Rescan()
	}
}

// Defaults for a desktop user: everything under home except caches, build
// output and dependency trees.
func DefaultConfig() Config {
	home := os.Getenv("HOME")
	var watch []string
	for _, d := range []string{"", "Downloads", "Desktop", "Documents", "Pictures", "Pictures/Screenshots", "Videos", "Music"} {
		if info, err := os.Stat(filepath.Join(home, d)); err == nil && info.IsDir() {
			watch = append(watch, filepath.Join(home, d))
		}
	}
	return Config{
		Roots: []string{home},
		Exclude: []string{
			".git", ".cache", "go/pkg", "node_modules", ".cargo", ".rustup", ".npm", ".bun",
			".local/share", ".local/state", ".mozilla", ".gradle", ".java", "__pycache__",
			".venv", ".pnpm-store", "target", ".steam", ".wine", "snap", ".pub-cache", ".android",
			"Android/Sdk", "go/pkg", ".dartServer", ".m2",
			// browser and Electron profiles: thousands of cache files each
			".config/chromium", ".config/google-chrome", ".config/BraveSoftware", ".config/vivaldi",
			".config/microsoft-edge", ".config/Claude", ".config/discord", ".config/Code",
			".config/Slack", ".config/Signal", ".config/obsidian", ".config/Electron",
		},
		Watch: watch,
	}
}

// buildTrees are folders full of generated or vendored files: a pdf in a
// vcpkg buildtree is rarely the one wanted.
var buildTrees = map[string]bool{
	"build": true, "builds": true, "buildtrees": true, "vendor": true, "third_party": true,
	"thirdparty": true, "dist": true, "deps": true, "site-packages": true, "vcpkg": true,
	"_deps": true, "obj": true,
}

// isBuried: a dot folder or a build tree anywhere in rel.
func isBuried(rel string) bool {
	for part := range strings.SplitSeq(rel, "/") {
		if strings.HasPrefix(part, ".") || buildTrees[strings.ToLower(part)] || strings.HasPrefix(strings.ToLower(part), "cmake-build") {
			return true
		}
	}
	return false
}

func trigram(s string) uint32 {
	return uint32(s[0])<<16 | uint32(s[1])<<8 | uint32(s[2])
}

// span is the run of entries under the given directories: [lo, hi).
func (ix *index) span(dirs []string) (lo, hi int) {
	lo, hi = len(ix.entries), 0
	at := func(path string) int {
		k, _ := slices.BinarySearchFunc(ix.sortedDirs, path, func(d uint32, p string) int { return strings.Compare(ix.dirs[d], p) })
		return k
	}
	for _, want := range dirs {
		// the folder itself, then every path from want+"/" up to want+"0",
		// the byte after '/'
		for _, d := range ix.sortedDirs[at(want):at(want+"0")] {
			if path := ix.dirs[d]; path == want || strings.HasPrefix(path, want+"/") {
				lo = min(lo, int(ix.firstEntry[d]))
				hi = max(hi, int(ix.lastEntry[d])+1)
			}
		}
	}
	return lo, hi
}

// Boosts are points for files the user opens, by absolute path; the engine
// sets them from its launch history and the desktop's recent files.
var boosts atomic.Pointer[map[string]int]

func SetBoosts(points map[string]int) {
	boosts.Store(&points)
}

type resolvedBoost struct {
	from    *map[string]int
	entries map[int32]int
}

// boostOf is entry i's points. The path list is resolved to entries the
// first time a search sees it with this index: a folder lookup by binary
// search, then a scan of that folder's own run.
func (ix *index) boostOf(i int32) int {
	src := boosts.Load()
	if src == nil {
		return 0
	}
	r := ix.boost.Load()
	if r == nil || r.from != src {
		r = &resolvedBoost{from: src, entries: map[int32]int{}}
		for path, points := range *src {
			if e, ok := ix.find(path); ok {
				r.entries[e] = points
			}
		}
		ix.boost.Store(r)
	}
	return r.entries[i]
}

// find is the entry for an absolute path.
func (ix *index) find(path string) (int32, bool) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	k, found := slices.BinarySearchFunc(ix.sortedDirs, dir, func(d uint32, p string) int { return strings.Compare(ix.dirs[d], p) })
	if !found {
		return 0, false
	}
	d := ix.sortedDirs[k]
	for e := ix.firstEntry[d]; e <= ix.lastEntry[d] && int(e) < len(ix.entries); e++ {
		if ix.entries[e].dir == int32(d) && ix.origName(int(e)) == name {
			return int32(e), true
		}
	}
	return 0, false
}
