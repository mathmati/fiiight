//go:build js

// WebAudio implementation TODO — currently silent.
// JSSpeaker satisfies AudioSink so the engine boots; a later change replaces
// the internals with a WebAudio-backed pump while keeping the type name.
package main

import (
	"sync"

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
}

func (s *JSSpeaker) Init(sampleRate beep.SampleRate, bufferSize int) error {
	s.sampleRate = sampleRate
	s.bufferSize = bufferSize
	s.mixer = &beep.Mixer{}
	return nil
}

func (s *JSSpeaker) Play(st beep.Streamer) {
	s.mu.Lock()
	s.mixer.Add(st)
	s.mu.Unlock()
}

func (s *JSSpeaker) Lock()   { s.mu.Lock() }
func (s *JSSpeaker) Unlock() { s.mu.Unlock() }

func (s *JSSpeaker) Close() {}

func (s *JSSpeaker) FillAudio() {
	// No output device yet; audio is silently discarded.
}
