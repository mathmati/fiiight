//go:build js

// Module music (.xm/.mod/.it/.s3m) requires libxmp (cgo), which is
// unavailable on js/wasm. This stub keeps sound.go compiling; attempting to
// play module music reports an error instead.
package main

import (
	"io"

	"github.com/gopxl/beep/v2"
)

func xmpDecode(f io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	return nil, beep.Format{}, Error("module music (xm/mod/it/s3m) is not supported in the browser build")
}
