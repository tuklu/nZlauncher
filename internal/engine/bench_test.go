package engine

import (
	"path/filepath"
	"testing"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/icons"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/sysinfo"
)

// Run against the real machine: go test ./internal/engine -bench . -benchtime 20x
// Each stage is what one keystroke pays today.

func BenchmarkQuery(b *testing.B) {
	for _, q := range []string{"", "fire", "set volume 40"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				Query(q)
			}
		})
	}
}

func BenchmarkCatalog(b *testing.B) {
	for b.Loop() {
		config.Load()
	}
}

func BenchmarkApps(b *testing.B) {
	settings, _ := config.LoadSettings()
	lib := icons.Load(filepath.Join(config.Dir(), "icons"), settings.IconLibrary)
	for b.Loop() {
		providers.Apps(lib)
	}
}

func BenchmarkEvents(b *testing.B) {
	for b.Loop() {
		learn.LoadEvents()
	}
}

func BenchmarkModel(b *testing.B) {
	events := learn.LoadEvents()
	for b.Loop() {
		learn.Stats(events)
		learn.BuildModel(events)
	}
}

func BenchmarkWindows(b *testing.B) {
	for b.Loop() {
		providers.Windows()
	}
}

func BenchmarkSSID(b *testing.B) {
	for b.Loop() {
		sysinfo.CurrentSSID()
	}
}

func BenchmarkClipboard(b *testing.B) {
	for b.Loop() {
		providers.Clipboard("")
	}
}

func BenchmarkBluetooth(b *testing.B) {
	for b.Loop() {
		providers.Bluetooth("")
	}
}
