//go:build js

// Background-video stub for the browser build. Stage/bgdef background
// videos are not supported yet (video_ffmpeg.go needs cgo/ffmpeg); Open
// returns an error so stages fall back to not playing the video.
package main

import "fmt"

type bgVideo struct {
	texture   Texture
	scaleMode BgVideoScaleMode
}

func (bgv *bgVideo) Open(filename string, volume int, sm BgVideoScaleMode, sf BgVideoScaleFilter, loop bool) error {
	bgv.scaleMode = sm
	return fmt.Errorf("background video playback is not supported in the browser build: %v", filename)
}

func (bgv *bgVideo) Tick() error {
	return nil
}

func (bgv *bgVideo) SetPlaying(on bool) {}

func (bgv *bgVideo) SetVisible(on bool) {}

func (bgv *bgVideo) Reset() {}

func (bgv *bgVideo) Close() {}

func (bgv *bgVideo) MixerCleared() {}
