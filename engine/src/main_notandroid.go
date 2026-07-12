//go:build !android

package main

// No-op twins of the Android-only startup helpers (see main_android.go).
// They are only invoked behind runtime.GOOS == "android" checks, so they are
// never reached on other platforms; they exist to keep main.go portable.
func androidInit() bool { return true }

func androidInitSubSystems() {}
