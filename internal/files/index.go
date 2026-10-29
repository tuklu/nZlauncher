// Package files keeps an in-memory index of the files under the user's
// roots, so a file query is a scan of memory instead of a walk of the disk.
// The daemon builds it in the background, rescans it every few minutes, and
// watches the directories where new files usually land so those appear at
// once. Without the daemon nobody builds it and callers fall back to fd.
package files

import (
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
)

// A full walk of a large home takes seconds of CPU; the watched directories
// cover where new files usually appear in between.
const rescanEvery = 20 * time.Minute

// Entry is one file. Its directory is shared with its siblings.
type Entry struct {
	dir     int32 // index into index.dirs
	Name    string
	Size    int64
	ModTime int64 // unix seconds
}

type index struct {
	dirs    []string
	entries []Entry
	built   time.Time
	// every lowercased name, each followed by '\n'; entry i starts at offsets[i]
	names   []byte
	offsets []uint32
	freq    [256]int // how often each byte occurs in names
	// trigram of a lowercased name -> span of postings: the entries
	// containing it, ascending. One flat array, so no per-list slack.
	trigrams map[uint32][2]uint32
	postings []uint32
}

func (ix *index) path(e *Entry) string {
	return filepath.Join(ix.dirs[e.dir], e.Name)
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
	ix := &index{built: time.Now()}
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
			}
			e := Entry{dir: idx, Name: d.Name(), Size: info.Size(), ModTime: info.ModTime().Unix()}
			l := strings.ToLower(e.Name)
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
			ix.entries = append(ix.entries, e)
			return nil
		})
	}
	ix.trigrams = make(map[uint32][2]uint32, len(lists))
	total := 0
	for _, p := range lists {
		total += len(p)
	}
	ix.postings = make([]uint32, 0, total)
	for t, p := range lists {
		ix.trigrams[t] = [2]uint32{uint32(len(ix.postings)), uint32(len(p))}
		ix.postings = append(ix.postings, p...)
		delete(lists, t) // free each list as soon as it is copied
	}
	return ix
}

// lowerName is entry i's name as stored in the packed buffer.
func (ix *index) lowerName(i int) string {
	end := len(ix.names) - 1
	if i+1 < len(ix.offsets) {
		end = int(ix.offsets[i+1]) - 1
	}
	return string(ix.names[ix.offsets[i]:end])
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

func trigram(s string) uint32 {
	return uint32(s[0])<<16 | uint32(s[1])<<8 | uint32(s[2])
}
