package platform

import (
	"testing"
)

// Run against the real machine: go test ./internal/platform -run Live -v
func TestLiveDetect(t *testing.T) {
	t.Logf("%+v", Detect())
}

func TestOverrides(t *testing.T) {
	Configure(map[string]string{"wifi": "networkmanager"})
	defer Configure(nil)
	if got := Current().Wifi; got != "networkmanager" {
		t.Fatalf("override ignored: %q", got)
	}
}

func BenchmarkDetect(b *testing.B) {
	for b.Loop() {
		Detect()
	}
}
