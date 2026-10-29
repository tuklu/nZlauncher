package tweaks

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/platform"
)

// Nightlight through hyprsunset's socket. "identity" (no tint) still reports
// 6000 K, so anything warmer than that counts as on.
const (
	neutralKelvin = 6000
	warmKelvin    = 4000
)

var ErrNoNightlight = errors.New("no nightlight nzlauncher can drive (hyprsunset on Hyprland)")

func sunsetSocket() string {
	return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "hypr", os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"), ".hyprsunset.sock")
}

func sunset(request string) (string, error) {
	conn, err := net.DialTimeout("unix", sunsetSocket(), 200*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := io.WriteString(conn, request); err != nil {
		return "", err
	}
	reply, err := io.ReadAll(conn)
	return strings.TrimSpace(string(reply)), err
}

// Nightlight is the current colour temperature; 0 when hyprsunset is not
// running, which is off too. ok is false without a supported backend.
func Nightlight() (kelvin int, ok bool) {
	if platform.Current().Nightlight != "hyprsunset" {
		return 0, false
	}
	reply, err := sunset("temperature")
	if err != nil {
		return 0, true
	}
	k, _ := strconv.Atoi(reply)
	return k, true
}

// NightlightOn reports whether a temperature is a tint.
func NightlightOn(kelvin int) bool {
	return kelvin > 0 && kelvin < neutralKelvin
}

// SetNightlight warms the screen to kelvin, or clears the tint with 0.
// hyprsunset is started when it is not running and kept for the next change.
func SetNightlight(kelvin int) error {
	if platform.Current().Nightlight != "hyprsunset" {
		return ErrNoNightlight
	}
	request := "identity"
	if kelvin > 0 {
		request = "temperature " + strconv.Itoa(kelvin)
	}
	if _, err := sunset(request); err == nil {
		return nil
	}
	if kelvin == 0 {
		return nil // not running is no tint already
	}
	cmd := exec.Command("hyprsunset", "-t", strconv.Itoa(kelvin))
	cmd.SysProcAttr = detached()
	return cmd.Start()
}
