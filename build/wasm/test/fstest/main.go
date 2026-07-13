//go:build js && wasm

// fstest exercises the browser fs-shim through Go's os package.
// It is mounted by build/wasm/test/index.html after the generated test content.zip is
// extracted to /ikemen and the cwd is set to /ikemen.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
)

func check(name string, ok bool, detail string) {
	if ok {
		fmt.Println("PASS: " + name)
	} else {
		fmt.Println("FAIL: " + name + " -- " + detail)
	}
}

func main() {
	// os.Getwd
	wd, err := os.Getwd()
	check("getwd", err == nil && wd == "/ikemen", fmt.Sprintf("wd=%q err=%v", wd, err))

	// os.ReadFile, exact case
	b, err := os.ReadFile("data/hello.txt")
	check("readfile-exact-case", err == nil && string(b) == "hello wasm\n", fmt.Sprintf("b=%q err=%v", b, err))

	// os.ReadFile with WRONG case in every component (MUGEN-style)
	b, err = os.ReadFile("DATA/Hello.TXT")
	check("readfile-wrong-case", err == nil && string(b) == "hello wasm\n", fmt.Sprintf("b=%q err=%v", b, err))

	// wrong case on a nested (deflated) entry
	b, err = os.ReadFile("Data/SUB/Nested.TxT")
	check("readfile-wrong-case-nested", err == nil && string(b) == "nested content: deflate me deflate me deflate me\n", fmt.Sprintf("b=%q err=%v", b, err))

	// missing file must be a real ENOENT
	_, err = os.ReadFile("data/definitely-missing.txt")
	check("readfile-enoent", os.IsNotExist(err), fmt.Sprintf("err=%v", err))

	// mkdir + write + re-read under /save/
	err = os.MkdirAll("/ikemen/save", 0o755)
	check("mkdir-save", err == nil, fmt.Sprintf("err=%v", err))
	err = os.WriteFile("/ikemen/save/test.txt", []byte("persist-me"), 0o644)
	check("writefile-save", err == nil, fmt.Sprintf("err=%v", err))
	b, err = os.ReadFile("/ikemen/save/test.txt")
	check("reread-save", err == nil && string(b) == "persist-me", fmt.Sprintf("b=%q err=%v", b, err))

	// os.ReadDir
	entries, err := os.ReadDir("data")
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	check("readdir", err == nil && len(names) == 3 && names[0] == "empty.bin" && names[1] == "hello.txt" && names[2] == "sub",
		fmt.Sprintf("names=%v err=%v", names, err))
	if err == nil && len(entries) == 3 {
		var subDir bool
		for _, e := range entries {
			if e.Name() == "sub" {
				subDir = e.IsDir()
			}
		}
		check("readdir-types", subDir, "sub not reported as dir")
	}

	// os.Stat
	fi, err := os.Stat("data/hello.txt")
	check("stat-file", err == nil && fi.Size() == int64(len("hello wasm\n")) && !fi.IsDir(),
		fmt.Sprintf("fi=%+v err=%v", fi, err))
	fi, err = os.Stat("data")
	check("stat-dir", err == nil && fi.IsDir(), fmt.Sprintf("err=%v", err))

	// seek + partial read through a plain os.File
	f, err := os.Open("data/sub/nested.txt")
	if err != nil {
		check("seek-read", false, err.Error())
	} else {
		_, err = f.Seek(7, io.SeekStart)
		buf := make([]byte, 7)
		_, err2 := io.ReadFull(f, buf)
		check("seek-read", err == nil && err2 == nil && string(buf) == "content", fmt.Sprintf("buf=%q err=%v/%v", buf, err, err2))
		f.Close()
	}

	// append mode
	err = os.WriteFile("/ikemen/save/app.txt", []byte("a"), 0o644)
	if err == nil {
		var af *os.File
		af, err = os.OpenFile("/ikemen/save/app.txt", os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			_, err = af.Write([]byte("b"))
			af.Close()
		}
	}
	b, err2 := os.ReadFile("/ikemen/save/app.txt")
	check("append", err == nil && err2 == nil && string(b) == "ab", fmt.Sprintf("b=%q err=%v/%v", b, err, err2))

	// rename + unlink
	err = os.Rename("/ikemen/save/app.txt", "/ikemen/save/app2.txt")
	b, err2 = os.ReadFile("/ikemen/save/app2.txt")
	check("rename", err == nil && err2 == nil && string(b) == "ab", fmt.Sprintf("b=%q err=%v/%v", b, err, err2))
	err = os.Remove("/ikemen/save/app2.txt")
	_, err2 = os.Stat("/ikemen/save/app2.txt")
	check("unlink", err == nil && os.IsNotExist(err2), fmt.Sprintf("err=%v/%v", err, err2))

	// persistence across page reloads: the previous run (if any) wrote
	// /ikemen/save/persist.txt, which is not in the zip; after a reload it
	// must have been restored from localStorage by mountZip.
	if pb, perr := os.ReadFile("/ikemen/save/persist.txt"); perr == nil && string(pb) == "round2" {
		fmt.Println("PASS: persisted-from-previous-run")
	} else {
		fmt.Println("INFO: no previous save found (expected on first run)")
	}
	err = os.WriteFile("/ikemen/save/persist.txt", []byte("round2"), 0o644)
	check("writefile-persist", err == nil, fmt.Sprintf("err=%v", err))

	fmt.Println("FSTEST-DONE")
}
