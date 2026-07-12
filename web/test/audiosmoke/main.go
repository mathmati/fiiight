//go:build js

// Standalone WebAudio smoke test for the js/wasm audio pipeline.
//
// Reimplements (inline, no engine import) the same output path as
// src/audio_js.go JSSpeaker: an AudioContext requested at 44100 Hz, a pull
// pump that ships 2048-frame chunks as float32 AudioBuffers scheduled
// back-to-back on ctx.currentTime with a ~4-chunk lead throttle. Instead of
// the game mixer, the source is a 440 Hz sine for 0.5 s. An AnalyserNode
// inserted before the destination verifies non-silent output (peak RMS above
// a threshold) and the test prints PASS/FAIL into window.__ikemenBootLog.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
	"time"
)

const (
	chunkFrames = 2048
	wantRate    = 44100
	sineHz      = 440.0
	sineSecs    = 0.5
	rmsThresh   = 0.05
)

func logLine(s string) {
	js.Global().Get("console").Call("log", s)
	blog := js.Global().Get("__ikemenBootLog")
	if !blog.IsUndefined() {
		blog.Call("push", s)
	}
}

func done() {
	js.Global().Set("__testDone", js.ValueOf(true))
}

func jsTry(f func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	f()
	return true
}

func main() {
	defer done()

	ctor := js.Global().Get("AudioContext")
	if ctor.IsUndefined() {
		logLine("FAIL: AudioContext not available")
		return
	}
	var ctx js.Value
	if !jsTry(func() { ctx = ctor.New(map[string]interface{}{"sampleRate": wantRate}) }) {
		if !jsTry(func() { ctx = ctor.New() }) {
			logLine("FAIL: could not create AudioContext")
			return
		}
	}
	rate := ctx.Get("sampleRate").Float()
	logLine(fmt.Sprintf("INFO: AudioContext rate=%v state=%s", rate, ctx.Get("state").String()))

	// The test runs with --autoplay-policy=no-user-gesture-required, but
	// resume anyway and wait briefly for "running".
	noop := js.FuncOf(func(js.Value, []js.Value) interface{} { return nil })
	jsTry(func() { ctx.Call("resume").Call("catch", noop) })
	deadline := time.Now().Add(3 * time.Second)
	for ctx.Get("state").String() != "running" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if ctx.Get("state").String() != "running" {
		logLine("FAIL: AudioContext never reached running state (state=" + ctx.Get("state").String() + ")")
		return
	}

	analyser := ctx.Call("createAnalyser")
	analyser.Set("fftSize", chunkFrames)
	analyser.Call("connect", ctx.Get("destination"))

	// Persistent scratch, same shape as JSSpeaker.
	chanBytes := make([]byte, chunkFrames*4)
	jsU8 := js.Global().Get("Uint8Array").New(chunkFrames * 4)
	jsF32 := js.Global().Get("Float32Array").New(jsU8.Get("buffer"))

	totalFrames := int(sineSecs * rate)
	phase := 0.0
	phaseInc := 2 * math.Pi * sineHz / rate
	generated := 0
	nextTime := 0.0
	chunkDur := float64(chunkFrames) / rate

	// Analyser sampling state.
	anaF32 := js.Global().Get("Float32Array").New(chunkFrames)
	anaU8 := js.Global().Get("Uint8Array").New(anaF32.Get("buffer"))
	anaBytes := make([]byte, chunkFrames*4)
	maxRMS := 0.0
	sampleRMS := func() {
		analyser.Call("getFloatTimeDomainData", anaF32)
		js.CopyBytesToGo(anaBytes, anaU8)
		sum := 0.0
		for i := 0; i < chunkFrames; i++ {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(anaBytes[i*4:])))
			sum += v * v
		}
		if rms := math.Sqrt(sum / chunkFrames); rms > maxRMS {
			maxRMS = rms
		}
	}

	// Pump loop: 17 ms cadence, one chunk per call, skip when ~4 chunks are
	// buffered ahead — mirrors JSSpeaker.FillAudio.
	start := time.Now()
	for time.Since(start) < 2*time.Second {
		now := ctx.Get("currentTime").Float()
		if generated < totalFrames {
			if nextTime < now {
				nextTime = now
			}
			if nextTime-now <= 4*chunkDur {
				n := chunkFrames
				if rem := totalFrames - generated; rem < n {
					n = rem
				}
				for i := 0; i < n; i++ {
					bits := math.Float32bits(float32(0.5 * math.Sin(phase)))
					binary.LittleEndian.PutUint32(chanBytes[i*4:], bits)
					phase += phaseInc
				}
				ab := ctx.Call("createBuffer", 2, n, rate)
				js.CopyBytesToJS(jsU8, chanBytes[:n*4])
				view := jsF32
				if n != chunkFrames {
					view = jsF32.Call("subarray", 0, n)
				}
				ab.Call("copyToChannel", view, 0)
				ab.Call("copyToChannel", view, 1)
				src := ctx.Call("createBufferSource")
				src.Set("buffer", ab)
				src.Call("connect", analyser)
				src.Call("start", nextTime)
				nextTime += float64(n) / rate
				generated += n
			}
		} else if now > nextTime+0.1 {
			break // everything played out
		}
		sampleRMS()
		time.Sleep(17 * time.Millisecond)
	}
	sampleRMS()

	logLine(fmt.Sprintf("INFO: generated=%d frames, peak RMS=%.4f (threshold %.2f)", generated, maxRMS, rmsThresh))
	if generated >= totalFrames && maxRMS > rmsThresh {
		logLine(fmt.Sprintf("PASS: 440Hz sine audible via WebAudio pipeline (rms=%.4f)", maxRMS))
	} else {
		logLine(fmt.Sprintf("FAIL: silent or incomplete output (generated=%d/%d rms=%.4f)", generated, totalFrames, maxRMS))
	}
}
