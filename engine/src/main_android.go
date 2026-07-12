//go:build android

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/veandco/go-sdl2/sdl"
)

// androidInit performs the Android-specific startup work that used to live in
// realMain (SDL GL attributes, baseDir checks and SDL init).
// It returns false when startup must be aborted.
func androidInit() bool {
	Logcat("Inside realMain...")
	runtime.LockOSThread()
	sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_ES)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 3)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 2)
	sdl.GLSetAttribute(sdl.GL_DOUBLEBUFFER, 1)
	sdl.GLSetAttribute(sdl.GL_ALPHA_SIZE, 0)
	sdl.GLSetAttribute(sdl.GL_DEPTH_SIZE, 24)
	// sdl.SetHint("SDL_VIDEO_EXTERNAL_CONTEXT", "0")
	// sdl.SetHint("SDL_HIDAPI_IGNORE_DEVICES", "1")
	// sdl.SetHint("SDL_JOYSTICK_ALLOW_BACKGROUND_EVENTS", "1")
	// sdl.SetHint(sdl.HINT_ORIENTATIONS, "LandscapeLeft LandscapeRight")
	// sdl.SetHint("SDL_ANDROID_TRAP_BACK_BUTTON", "1")
	// sdl.SetHint("SDL_JOYSTICK_HIDAPI", "0")
	// sdl.SetHint("SDL_ANDROID_SEPARATE_MOUSE_AND_TOUCH", "1")

	if sys.baseDir == "" {
		panic("FATAL: Android baseDir not set")
	}

	Logcat("sys.baseDir is: " + sys.baseDir)

	// Check if the directory even exists to Go
	if info, err := os.Stat(sys.baseDir); err != nil {
		Logcat(fmt.Sprintf("LOG: STAT ERROR: %v\n", err))
	} else {
		Logcat(fmt.Sprintf("LOG: STAT OK: %s is a dir: %v\n", sys.baseDir, info.IsDir()))
	}

	// FIX 1: Explicitly initialize os.Args before processCommandLine
	if os.Args == nil || len(os.Args) == 0 {
		os.Args = []string{"ikemen-go"}
	}

	if err := os.Chdir(sys.baseDir); err != nil {
		Logcat(fmt.Sprintf("LOG: CHDIR FAILED: %v\n", err))
		// Don't panic yet, let's see if we can continue
	} else {
		Logcat("LOG: CHDIR SUCCESSFUL")
	}

	// Init SDL NOW
	if err := sdl.Init(sdl.INIT_AUDIO | sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_TIMER); err != nil {
		Logcat("LOG: SDL Init Failed: " + err.Error())
		return false
	}
	Logcat("LOG: SDL Init SUCCESS")
	return true
}

// androidInitSubSystems initializes SDL joystick and game controller subsystems.
func androidInitSubSystems() {
	sdl.InitSubSystem(sdl.INIT_JOYSTICK)
	sdl.InitSubSystem(sdl.INIT_GAMECONTROLLER)
	Logcat("LOG: Subsystems initialized!")
}
