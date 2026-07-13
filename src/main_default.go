//go:build !android

package main

// Default implementations of the platform startup seam (see main.go and
// main_android.go). Desktop and js/wasm builds run from the working
// directory and need no extra subsystem setup.

// platformInit performs platform-specific startup work before anything else
// in realMain. It returns false when startup must be aborted.
func platformInit() bool {
	sys.baseDir = "./"
	return true
}

// platformInitSubSystems initializes platform input subsystems after the
// command line and stats files have been processed.
func platformInitSubSystems() {}
