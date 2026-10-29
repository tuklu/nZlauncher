package providers

import "sync/atomic"

// enabled reports whether config.toml leaves a provider on. The engine sets
// it whenever the settings change; until then everything is on.
var enabled atomic.Pointer[func(string) bool]

// Configure installs the check every provider consults first.
func Configure(on func(name string) bool) {
	enabled.Store(&on)
}

func On(name string) bool {
	if f := enabled.Load(); f != nil {
		return (*f)(name)
	}
	return true
}
