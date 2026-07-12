//go:build js

package main

import (
	"fmt"
	"io"
	"os"
	"syscall/js"
)

// Log writer implementation
func NewLogWriter() io.Writer {
	return os.Stderr
}

// Message box implementation. Uses the host page's overlay hook when
// present (window.__ikemenShowError), otherwise falls back to alert().
func ShowInfoDialog(message, title string) {
	js.Global().Get("console").Call("log", title+": "+message)
	showBrowserDialog(title + "\n\n" + message)
}

func ShowErrorDialog(message string) {
	js.Global().Get("console").Call("error", message)
	showBrowserDialog(message)
}

func showBrowserDialog(message string) {
	global := js.Global()
	if hook := global.Get("__ikemenShowError"); hook.Truthy() {
		hook.Invoke(message)
		return
	}
	if global.Get("alert").Truthy() {
		global.Call("alert", message)
	}
}

// TTF font loading (util_desktop.go minus the OS font-directory fallback)
func LoadFntTtf(f *Fnt, fontfile string, filename string, height int32) {
	// Search in local directory
	fileDir := SearchFile(filename, []string{fontfile, sys.motif.Def, "", "data/"}, "font/")
	if fp := FileExist(fileDir); len(fp) == 0 {
		panic(fmt.Errorf("failed to find ttf font %v", fileDir))
	}
	// Load ttf
	if height == -1 {
		height = int32(f.Size[1])
	} else {
		f.Size[1] = uint16(height)
	}
	ttf, err := gfxFont.LoadFont(fileDir, height, int(sys.gameWidth), int(sys.gameHeight))
	if err != nil {
		panic(fmt.Errorf("failed to load ttf font %v: %w", fileDir, err))
	}
	f.ttf = ttf.(Font)

	// Create Ttf dummy palettes
	f.palettes = make([][256]uint32, 1)
	for i := 0; i < 256; i++ {
		f.palettes[0][i] = 0
	}
}

func selectRenderer(cfgVal string) (Renderer, FontRenderer) {
	// The browser build always renders through WebGL2.
	return &Renderer_WebGL2{}, &FontRenderer_WebGL2{}
}

func Logcat(s string) {
	fmt.Println(s)
}

// osPreferredLanguage returns the browser UI language (GOOS=js matches no
// util_<os>.go file, so the js backend provides its own implementation).
func osPreferredLanguage() string {
	navigator := js.Global().Get("navigator")
	if navigator.Truthy() {
		if lang := navigator.Get("language"); lang.Truthy() {
			return lang.String()
		}
	}
	return ""
}
