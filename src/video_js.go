//go:build js

// Browser (js/wasm) background-video backend. The native backend
// (video_ffmpeg.go) decodes with FFmpeg and routes audio through the Go
// mixer; here the browser does both instead: the video file's bytes are
// read from the in-memory zip fs, handed to JS as a Blob URL and played
// on a hidden, detached <video> element. Each Tick, when the element has
// a new frame, it is uploaded straight to the WebGL2 texture via
// texImage2D(HTMLVideoElement) (Texture_WebGL2.SetDataFromTexImageSource),
// so there is no per-frame CPU pixel copy. Audio plays from the <video>
// element itself, not through the Go mixer.
//
// Scale modes: the native backend pre-scales/pads/crops frames to the
// window with an FFmpeg filter graph (buildFFFilterGraph). The browser
// backend always uploads frames at the video's native resolution, which
// matches native SM_None semantics exactly ("draw at native resolution");
// the frame is then scaled by the normal background draw path according
// to the motif/storyboard localcoord, which is what the stock logo
// storyboard (1280x720 video, localcoord 1280,720) uses. The other modes
// (Stretch/Fit/FitWidth/FitHeight/ZoomFill) are NOT implemented and fall
// back to SM_None with a log message.
//
// Autoplay policy: play() is attempted unmuted; if the browser rejects
// it, playback retries muted and a one-shot window pointerdown/keydown
// listener unmutes on the first user gesture (mirroring the WebAudio
// resume hook in audio_js.go).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall/js"
)

// HTMLMediaElement.readyState value HAVE_CURRENT_DATA: data for the
// current playback position is available for upload.
const videoHaveCurrentData = 2

type bgVideo struct {
	texture   Texture
	scaleMode BgVideoScaleMode
	el        js.Value // hidden, detached <video> element
	blobURL   js.Value // URL.createObjectURL result; revoked in Close
	loop      bool
	playing   bool
	visible   bool
	// inMixer mirrors the native mixer attachment: after MixerCleared the
	// audio stays muted until SetPlaying(true) "re-attaches" it.
	inMixer bool
	// gestureMuted is set when autoplay with sound was rejected; audio
	// stays muted until the first user gesture.
	gestureMuted bool
	hookOn       bool    // gesture listeners currently registered
	lastTime     float64 // el.currentTime of the last uploaded frame
	playFail     js.Func // rejection handler for el.play()
	gestureCb    js.Func // one-shot unmute listener
	opened       bool
}

// videoMIMEType guesses the Blob MIME type from the file extension.
// Browsers content-sniff media data anyway, so this is only a hint.
func videoMIMEType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".webm":
		return "video/webm"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".ogv", ".ogg":
		return "video/ogg"
	}
	return "video/mp4"
}

func (bgv *bgVideo) Open(filename string, volume int, sm BgVideoScaleMode, sf BgVideoScaleFilter, loop bool) error {
	// The wasm fs shim serves the content zip through the os package.
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	bgv.scaleMode = sm
	bgv.loop = loop
	bgv.playing = false
	bgv.visible = true
	bgv.inMixer = true
	bgv.gestureMuted = false
	bgv.lastTime = -1
	bgv.texture = nil

	if sm != SM_None {
		// See the file comment: only SM_None is implemented in the browser.
		LogMessage("Video: scalemode %d is not supported in the browser build, using None (%s)", sm, filename)
	}
	// sf (scalefilter) selects an FFmpeg sws_scale kernel natively; the
	// browser leaves frame scaling to GPU sampling, so it is ignored.
	_ = sf

	global := js.Global()
	u8 := global.Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(u8, data)
	blob := global.Get("Blob").New(
		[]interface{}{u8},
		map[string]interface{}{"type": videoMIMEType(filename)},
	)
	bgv.blobURL = global.Get("URL").Call("createObjectURL", blob)

	el := global.Get("document").Call("createElement", "video")
	el.Set("playsInline", true)
	el.Call("setAttribute", "playsinline", "")
	el.Set("loop", loop)
	el.Set("preload", "auto")
	// Native maps volume through the BGM/master dB curve
	// (updateAudioVolume); the browser approximates with the element's
	// linear volume alone.
	el.Set("volume", float64(Clamp(int32(volume), 0, 100))/100)
	el.Set("src", bgv.blobURL)
	el.Call("load")
	bgv.el = el

	// Rejection handler: autoplay with sound was blocked. Retry muted and
	// unmute on the first user gesture.
	bgv.playFail = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if !bgv.opened || bgv.gestureMuted {
			// Already in the muted-retry state (or closed): swallow the
			// rejection instead of retrying forever.
			return nil
		}
		bgv.gestureMuted = true
		bgv.updateMuted()
		bgv.installGestureHook()
		if bgv.playing {
			bgv.tryPlay()
		}
		return nil
	})
	bgv.gestureCb = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if bgv.opened && bgv.gestureMuted {
			bgv.gestureMuted = false
			bgv.updateMuted()
			if bgv.playing {
				bgv.tryPlay()
			}
		}
		return nil
	})

	bgv.opened = true
	return nil
}

// updateMuted recomputes the element's muted flag from all audio gates.
func (bgv *bgVideo) updateMuted() {
	bgv.el.Set("muted", bgv.gestureMuted || !bgv.visible || !bgv.inMixer)
}

func (bgv *bgVideo) installGestureHook() {
	if bgv.hookOn {
		return
	}
	bgv.hookOn = true
	window := js.Global()
	opts := map[string]interface{}{"once": true}
	window.Call("addEventListener", "pointerdown", bgv.gestureCb, opts)
	window.Call("addEventListener", "keydown", bgv.gestureCb, opts)
}

func (bgv *bgVideo) removeGestureHook() {
	if !bgv.hookOn {
		return
	}
	bgv.hookOn = false
	window := js.Global()
	window.Call("removeEventListener", "pointerdown", bgv.gestureCb)
	window.Call("removeEventListener", "keydown", bgv.gestureCb)
}

func (bgv *bgVideo) tryPlay() {
	jsTry(func() {
		p := bgv.el.Call("play")
		if p.Truthy() {
			p.Call("catch", bgv.playFail)
		}
	})
}

func (bgv *bgVideo) Tick() error {
	if !bgv.opened {
		return nil
	}
	el := bgv.el
	if !bgv.loop && el.Get("ended").Bool() {
		// End of stream with loop=0: match the native backend — drop the
		// texture so the stage/storyboard stops drawing the video.
		bgv.texture = nil
		return nil
	}
	// While hidden the element keeps its clock running (like the native
	// pacing goroutine) but no frames are presented.
	if !bgv.visible {
		return nil
	}
	if el.Get("readyState").Int() < videoHaveCurrentData {
		return nil
	}
	w := int32(el.Get("videoWidth").Int())
	h := int32(el.Get("videoHeight").Int())
	if w <= 0 || h <= 0 {
		return nil
	}
	t := el.Get("currentTime").Float()
	if bgv.texture != nil && w == bgv.texture.GetWidth() && h == bgv.texture.GetHeight() && t == bgv.lastTime {
		// No new frame; the GPU texture retains the last upload, so no
		// per-tick re-upload is needed (unlike the native backend).
		return nil
	}
	if bgv.texture == nil || w != bgv.texture.GetWidth() || h != bgv.texture.GetHeight() {
		bgv.texture = gfx.newTexture(w, h, 32, true)
	}
	if tex, ok := bgv.texture.(*Texture_WebGL2); ok {
		tex.SetDataFromTexImageSource(el)
	}
	bgv.lastTime = t
	return nil
}

func (bgv *bgVideo) SetPlaying(on bool) {
	if !bgv.opened {
		return
	}
	// Like the native backend, never "resume" a finished loop=0 stream.
	if !bgv.loop && bgv.el.Get("ended").Bool() {
		bgv.playing = false
		return
	}
	if on == bgv.playing {
		return
	}
	bgv.playing = on
	if on {
		// Re-attach the audio gate (it may have been cleared by
		// MixerCleared), matching the native mixer re-add.
		bgv.inMixer = true
		bgv.updateMuted()
		bgv.tryPlay()
	} else {
		jsTry(func() { bgv.el.Call("pause") })
	}
}

// SetVisible controls whether decoded A/V is presented. While hidden,
// Tick stops uploading frames and the audio is muted (the native backend
// drops both), but the media clock keeps running.
func (bgv *bgVideo) SetVisible(on bool) {
	if !bgv.opened || on == bgv.visible {
		return
	}
	bgv.visible = on
	bgv.updateMuted()
}

// Reset rewinds to t=0 and drops the texture; playback state is left to
// the callers, which pair Reset with SetPlaying(false).
func (bgv *bgVideo) Reset() {
	if !bgv.opened {
		return
	}
	jsTry(func() { bgv.el.Set("currentTime", 0) })
	bgv.texture = nil
	bgv.lastTime = -1
}

// MixerCleared marks the audio path as detached (muted) until the next
// SetPlaying(true), mirroring the native sys.soundMixer.Clear() handling.
func (bgv *bgVideo) MixerCleared() {
	bgv.inMixer = false
	if bgv.opened {
		bgv.updateMuted()
	}
}

// Close stops playback and frees resources. Safe to call multiple times.
func (bgv *bgVideo) Close() {
	if !bgv.opened {
		return
	}
	bgv.opened = false
	bgv.playing = false
	jsTry(func() {
		bgv.el.Call("pause")
		bgv.el.Call("removeAttribute", "src")
		bgv.el.Call("load") // release the media resource
	})
	bgv.removeGestureHook()
	jsTry(func() { js.Global().Get("URL").Call("revokeObjectURL", bgv.blobURL) })
	bgv.playFail.Release()
	bgv.gestureCb.Release()
	bgv.el = js.Undefined()
	bgv.blobURL = js.Undefined()
	bgv.texture = nil
}
