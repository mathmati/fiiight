//go:build js

// WebAudio implementation of the final-mix audio sink.
//
// Design: pull model identical to SDLSpeaker — a 17ms pump goroutine calls
// FillAudio, which streams 2048-frame chunks from the internal beep.Mixer
// under the mutex and ships them to the browser. Output uses double-buffered
// AudioBufferSourceNode scheduling on AudioContext.currentTime (up to ~4
// chunks ahead) rather than an AudioWorklet: the producer is the wasm main
// thread either way, so a worklet ring buffer has the same underrun
// characteristics but adds an async addModule round-trip and a postMessage
// feedback protocol just to learn the queue depth. currentTime scheduling
// gives us the queue depth for free and keeps everything in this one file.
//
// Autoplay: the context is created before any user gesture and therefore
// usually starts "suspended". While suspended the sink keeps draining the
// mixer at real-time rate and discards the samples, so the game runs silently
// and never blocks; web/main.js calls window.__ikemenResumeAudio (registered
// here) on the first pointerdown/keydown, and on resume scheduling restarts
// from the current context clock — no stale buffers are queued while
// suspended, so audio joins cleanly.
package main

import (
	"encoding/binary"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"

	"github.com/gopxl/beep/v2"
)

type AudioSink interface {
	Init(sr beep.SampleRate, bufferSize int) error
	Play(s beep.Streamer)
	Lock()
	Unlock()
	Close()
	FillAudio()
}

var speaker AudioSink

// newSpeaker returns the platform audio sink (see system.go init).
func newSpeaker() AudioSink {
	return &JSSpeaker{}
}

type JSSpeaker struct {
	mixer      *beep.Mixer
	mu         sync.Mutex
	sampleRate beep.SampleRate
	bufferSize int
	buf        [][2]float64
	closed     atomic.Bool

	ctx     js.Value // AudioContext
	ctxRate float64  // ctx.sampleRate (authoritative output rate)

	// Per-channel scratch: Go byte slices holding little-endian float32
	// samples, mirrored into persistent JS Float32Arrays each chunk.
	chanBytes [2][]byte
	jsU8      [2]js.Value
	jsF32     [2]js.Value

	// End time (in AudioContext.currentTime seconds) of the last scheduled
	// buffer; 0 forces a reschedule from the current clock.
	nextTime float64

	// Real-time discard bookkeeping while the context is suspended.
	suspended  bool
	suspAnchor time.Time
	suspPulled int64

	resumeHook js.Func
	noopFunc   js.Func
}

// jsTry runs f and reports whether it completed without a JS exception.
func jsTry(f func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	f()
	return true
}

func (s *JSSpeaker) Init(sampleRate beep.SampleRate, bufferSize int) error {
	s.sampleRate = sampleRate
	s.bufferSize = bufferSize
	s.mixer = &beep.Mixer{}
	s.buf = make([][2]float64, bufferSize)

	ctor := js.Global().Get("AudioContext")
	if ctor.IsUndefined() {
		ctor = js.Global().Get("webkitAudioContext")
	}
	if ctor.IsUndefined() {
		return errors.New("WebAudio not available; audio disabled")
	}

	// Ask the context for the engine's configured rate so no resample is
	// needed (supported by all current browsers); fall back to the device
	// default if the browser rejects the option.
	if !jsTry(func() { s.ctx = ctor.New(map[string]interface{}{"sampleRate": int(sampleRate)}) }) {
		if !jsTry(func() { s.ctx = ctor.New() }) {
			s.ctx = js.Undefined()
			return errors.New("could not create AudioContext; audio disabled")
		}
	}
	s.ctxRate = s.ctx.Get("sampleRate").Float()

	// If the browser gave us a different rate, adopt it engine-wide. This is
	// safe here: speaker.Init runs from sys.init (system.go:469) before any
	// sound/BGM is loaded, and every consumer of sys.cfg.Sound.SampleRate
	// (per-source resampling in sound.go, the Normalizer coefficients) reads
	// it at stream time, after this point.
	if int32(s.ctxRate) != int32(sampleRate) && int32(s.ctxRate) > 0 {
		sys.cfg.Sound.SampleRate = int32(s.ctxRate)
		s.sampleRate = beep.SampleRate(int32(s.ctxRate))
	}

	// Persistent JS-side scratch (one Float32Array per channel, plus a byte
	// view for js.CopyBytesToJS).
	for c := 0; c < 2; c++ {
		s.chanBytes[c] = make([]byte, bufferSize*4)
		s.jsU8[c] = js.Global().Get("Uint8Array").New(bufferSize * 4)
		s.jsF32[c] = js.Global().Get("Float32Array").New(s.jsU8[c].Get("buffer"))
	}

	// Autoplay-policy resume hook: web/main.js calls this on the first user
	// gesture. Also try an immediate resume for contexts allowed to start.
	s.noopFunc = js.FuncOf(func(js.Value, []js.Value) interface{} { return nil })
	s.resumeHook = js.FuncOf(func(js.Value, []js.Value) interface{} {
		s.tryResume()
		return nil
	})
	js.Global().Set("__ikemenResumeAudio", s.resumeHook)
	s.tryResume()

	// Start the audio pump (same cadence as SDLSpeaker).
	SafeGo(func() {
		for {
			s.FillAudio()
			time.Sleep(time.Millisecond * 17)
		}
	})

	return nil
}

func (s *JSSpeaker) tryResume() {
	if s.closed.Load() || s.ctx.IsUndefined() {
		return
	}
	jsTry(func() {
		if s.ctx.Get("state").String() == "suspended" {
			// Swallow the promise rejection (e.g. still no user gesture).
			s.ctx.Call("resume").Call("catch", s.noopFunc)
		}
	})
}

func (s *JSSpeaker) FillAudio() {
	if s.closed.Load() || s.ctx.IsUndefined() {
		return
	}
	// A JS exception in the audio path must never take the game down.
	defer func() { _ = recover() }()

	state := s.ctx.Get("state").String()
	if state == "closed" {
		return
	}
	if state != "running" {
		// Suspended by autoplay policy: keep the game running silently by
		// draining the mixer at real-time rate and discarding the samples.
		s.discardRealtime()
		return
	}
	if s.suspended {
		// Just resumed: restart scheduling from the current clock so audio
		// joins cleanly (nothing stale was queued while suspended).
		s.suspended = false
		s.nextTime = 0
	}

	now := s.ctx.Get("currentTime").Float()
	chunkDur := float64(s.bufferSize) / s.ctxRate
	if s.nextTime < now {
		// Startup, resume, or main-thread stall: drop the stale schedule
		// point; the gap simply plays as silence.
		s.nextTime = now
	}
	// Queue-depth throttle, mirroring SDLSpeaker's GetQueuedAudioSize check:
	// skip pulling when ~4 chunks are already buffered ahead of playback.
	if s.nextTime-now > 4*chunkDur {
		return
	}

	frames := s.bufferSize
	s.mu.Lock()
	n, _ := s.mixer.Stream(s.buf[:frames])
	s.mu.Unlock()
	if n == 0 {
		return
	}

	// Deinterleave to per-channel little-endian float32 (JS typed arrays are
	// host-endian; wasm hosts are little-endian).
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(s.chanBytes[0][i*4:], math.Float32bits(clampF32(s.buf[i][0])))
		binary.LittleEndian.PutUint32(s.chanBytes[1][i*4:], math.Float32bits(clampF32(s.buf[i][1])))
	}

	ab := s.ctx.Call("createBuffer", 2, n, s.ctxRate)
	for c := 0; c < 2; c++ {
		js.CopyBytesToJS(s.jsU8[c], s.chanBytes[c][:n*4])
		if n == s.bufferSize {
			ab.Call("copyToChannel", s.jsF32[c], c)
		} else {
			ab.Call("copyToChannel", s.jsF32[c].Call("subarray", 0, n), c)
		}
	}
	src := s.ctx.Call("createBufferSource")
	src.Set("buffer", ab)
	src.Call("connect", s.ctx.Get("destination"))
	src.Call("start", s.nextTime)
	s.nextTime += float64(n) / s.ctxRate
}

// discardRealtime consumes the mixer at wall-clock rate while the context is
// suspended, so BGM/SFX positions track real time and audio picks up "now"
// when the user gesture arrives.
func (s *JSSpeaker) discardRealtime() {
	if !s.suspended {
		s.suspended = true
		s.suspAnchor = time.Now()
		s.suspPulled = 0
	}
	target := int64(time.Since(s.suspAnchor).Seconds() * float64(s.sampleRate))
	// Bound the catch-up work per call (e.g. after background-tab timer
	// throttling); if still behind afterwards, just re-anchor — the samples
	// are discarded anyway.
	for i := 0; i < 16 && s.suspPulled < target; i++ {
		chunk := s.bufferSize
		if rem := target - s.suspPulled; rem < int64(chunk) {
			chunk = int(rem)
		}
		if chunk <= 0 {
			break
		}
		s.mu.Lock()
		n, _ := s.mixer.Stream(s.buf[:chunk])
		s.mu.Unlock()
		if n == 0 {
			break
		}
		s.suspPulled += int64(n)
	}
	if s.suspPulled < target {
		s.suspAnchor = time.Now()
		s.suspPulled = 0
	}
}

func clampF32(v float64) float32 {
	if v > 1 {
		v = 1
	} else if v < -1 {
		v = -1
	}
	return float32(v)
}

func (s *JSSpeaker) Play(st beep.Streamer) {
	s.mu.Lock()
	s.mixer.Add(st)
	s.mu.Unlock()
}

func (s *JSSpeaker) Lock()   { s.mu.Lock() }
func (s *JSSpeaker) Unlock() { s.mu.Unlock() }

func (s *JSSpeaker) Close() {
	if s.closed.Swap(true) {
		return // idempotent
	}
	if !s.ctx.IsUndefined() {
		jsTry(func() {
			if s.ctx.Get("state").String() != "closed" {
				s.ctx.Call("close").Call("catch", s.noopFunc)
			}
		})
	}
}
