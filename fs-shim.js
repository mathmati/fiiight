// fs-shim.js -- in-memory POSIX-ish filesystem for Go js/wasm (wasm_exec.js).
// Provides globalThis.fs / globalThis.process / globalThis.path with the
// Node-style callback API that Go's syscall/fs_js.go calls, plus
// mountZip(url, mountPoint, onProgress) to seed the tree from a zip archive
// and mountZipIndex(url, mountPoint, opts) to mount lazily: only the zip's
// central directory is fetched up front, file bodies are Range-fetched and
// inflated on first open. Files under a "/save/" path segment are mirrored
// to localStorage and restored at mount time.
"use strict";
(() => {
	// Node-compatible mode bits; Go's js syscall uses the same values.
	const S_IFDIR = 0o040000;
	const S_IFREG = 0o100000;
	const S_IFCHR = 0o020000;

	const LS_PREFIX = "ikemenfs:";

	const enc = new TextEncoder();
	const dec = new TextDecoder();

	let inoCounter = 1;

	function makeDir(mode) {
		return { type: "dir", ino: inoCounter++, mode: mode == null ? 0o755 : mode & 0o777, children: new Map(), atimeMs: Date.now(), mtimeMs: Date.now(), ctimeMs: Date.now() };
	}
	function makeFile(data, mode) {
		const buf = data || new Uint8Array(0);
		return { type: "file", ino: inoCounter++, mode: mode == null ? 0o644 : mode & 0o777, buf, size: buf.length, atimeMs: Date.now(), mtimeMs: Date.now(), ctimeMs: Date.now() };
	}
	// Lazy file node: same shape as makeFile's, but the content still lives
	// in the zip on the server; `size` (from the central directory) serves
	// stat/readdir without a download, buf stays null until hydrate() runs.
	function makeLazyFile(lazy) {
		return { type: "file", ino: inoCounter++, mode: 0o644, buf: null, size: lazy.size, lazy, atimeMs: Date.now(), mtimeMs: Date.now(), ctimeMs: Date.now() };
	}
	function fileData(node) {
		if (node.buf === null) throw errnoError("EIO", "lazy file not hydrated");
		return node.buf.subarray(0, node.size);
	}
	function ensureCapacity(node, n) {
		if (node.buf.length >= n) return;
		let cap = Math.max(node.buf.length * 2, n, 64);
		const nb = new Uint8Array(cap);
		nb.set(node.buf.subarray(0, node.size));
		node.buf = nb;
	}

	const root = makeDir();
	let cwd = "/";

	// errno-style errors: Go's syscall maps err.code ("ENOENT", ...) via
	// errnoByCode; anything unknown becomes EINVAL-ish garbage, so the code
	// strings here must match syscall/tables_js.go.
	function errnoError(code, path) {
		const err = new Error(code + (path ? ": " + path : ""));
		err.code = code;
		return err;
	}

	// ---- path handling -------------------------------------------------
	function normalize(p) {
		const abs = p.startsWith("/");
		const out = [];
		for (const part of p.split("/")) {
			if (part === "" || part === ".") continue;
			if (part === "..") {
				if (out.length > 0 && out[out.length - 1] !== "..") out.pop();
				else if (!abs) out.push("..");
				continue;
			}
			out.push(part);
		}
		return (abs ? "/" : "") + out.join("/") || (abs ? "/" : ".");
	}
	function absPath(p) {
		p = String(p);
		if (!p.startsWith("/")) p = cwd + "/" + p;
		return normalize(p);
	}

	// Case-insensitive fallback: MUGEN content references files with
	// mismatched case, so each component tries an exact match first, then a
	// case-insensitive scan of the directory.
	function childLookup(dir, name) {
		const exact = dir.children.get(name);
		if (exact) return { name, node: exact };
		const lower = name.toLowerCase();
		for (const [n, node] of dir.children) {
			if (n.toLowerCase() === lower) return { name: n, node };
		}
		return null;
	}

	// Resolve absolute normalized path -> { node, parent, name, realPath }.
	// node is null when the final component doesn't exist (parent still set
	// if the parent dir exists).
	function lookup(p) {
		p = absPath(p);
		if (p === "/") return { node: root, parent: null, name: "", realPath: "/" };
		const parts = p.slice(1).split("/");
		let dir = root;
		let real = "";
		for (let i = 0; i < parts.length - 1; i++) {
			const hit = childLookup(dir, parts[i]);
			if (!hit) return { node: null, parent: null, name: parts[i], realPath: null, missingDir: true };
			if (hit.node.type !== "dir") return { node: null, parent: null, name: parts[i], realPath: null, notDir: true };
			dir = hit.node;
			real += "/" + hit.name;
		}
		const last = parts[parts.length - 1];
		const hit = childLookup(dir, last);
		if (!hit) return { node: null, parent: dir, name: last, realPath: real + "/" + last };
		return { node: hit.node, parent: dir, name: hit.name, realPath: real + "/" + hit.name };
	}

	function getNode(p, code) {
		const r = lookup(p);
		if (r.notDir) throw errnoError("ENOTDIR", p);
		if (!r.node) throw errnoError(code || "ENOENT", p);
		return r;
	}

	// ---- persistence (localStorage mirror of /save/ paths) --------------
	function b64FromBytes(bytes) {
		let s = "";
		for (let i = 0; i < bytes.length; i += 0x8000) {
			s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
		}
		return btoa(s);
	}
	function bytesFromB64(b64) {
		const s = atob(b64);
		const out = new Uint8Array(s.length);
		for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
		return out;
	}
	function isSavePath(p) {
		return p.includes("/save/");
	}
	function persist(path, node) {
		// Lazy nodes hold no content yet (and none was changed): skip.
		if (!isSavePath(path) || node.type !== "file" || node.lazy) return;
		try {
			localStorage.setItem(LS_PREFIX + path, b64FromBytes(fileData(node)));
		} catch (e) {
			console.warn("fs-shim: persist failed for " + path, e);
		}
	}
	function unpersist(path) {
		if (!isSavePath(path)) return;
		try { localStorage.removeItem(LS_PREFIX + path); } catch (e) { /* ignore */ }
	}
	function restoreSaves() {
		let n = 0;
		try {
			for (let i = 0; i < localStorage.length; i++) {
				const key = localStorage.key(i);
				if (!key || !key.startsWith(LS_PREFIX)) continue;
				const path = key.slice(LS_PREFIX.length);
				if (!isSavePath(path)) continue;
				writeFileAt(path, bytesFromB64(localStorage.getItem(key)), false);
				n++;
			}
		} catch (e) {
			console.warn("fs-shim: restore from localStorage failed", e);
		}
		return n;
	}

	// mkdir -p + set node, used by zip mounts and save restore.
	function setFileAt(path, node) {
		path = absPath(path);
		const parts = path.slice(1).split("/");
		let dir = root;
		for (let i = 0; i < parts.length - 1; i++) {
			const hit = childLookup(dir, parts[i]);
			if (hit) {
				if (hit.node.type !== "dir") throw errnoError("ENOTDIR", path);
				dir = hit.node;
			} else {
				const d = makeDir();
				dir.children.set(parts[i], d);
				dir = d;
			}
		}
		const name = parts[parts.length - 1];
		const hit = childLookup(dir, name);
		dir.children.set(hit ? hit.name : name, node);
		return path;
	}
	function writeFileAt(path, bytes, doPersist) {
		const node = makeFile(bytes);
		path = setFileAt(path, node);
		if (doPersist) persist(path, node);
	}
	function mkdirAt(path) {
		path = absPath(path);
		if (path === "/") return;
		const parts = path.slice(1).split("/");
		let dir = root;
		for (const part of parts) {
			const hit = childLookup(dir, part);
			if (hit) {
				if (hit.node.type !== "dir") throw errnoError("ENOTDIR", path);
				dir = hit.node;
			} else {
				const d = makeDir();
				dir.children.set(part, d);
				dir = d;
			}
		}
	}

	// ---- stat objects ----------------------------------------------------
	function statFor(node) {
		const isDir = node.type === "dir";
		const size = node.type === "file" ? node.size : 0;
		return {
			dev: 1,
			ino: node.ino,
			mode: (isDir ? S_IFDIR : node.type === "char" ? S_IFCHR : S_IFREG) | node.mode,
			nlink: 1,
			uid: 0,
			gid: 0,
			rdev: 0,
			size,
			blksize: 4096,
			blocks: Math.ceil(size / 512),
			atimeMs: node.atimeMs,
			mtimeMs: node.mtimeMs,
			ctimeMs: node.ctimeMs,
			isDirectory() { return isDir; },
			isFile() { return node.type === "file"; },
			isSymbolicLink() { return false; },
			isCharacterDevice() { return node.type === "char"; },
			isBlockDevice() { return false; },
			isFIFO() { return false; },
			isSocket() { return false; },
		};
	}
	const ttyNode = { type: "char", ino: 0, mode: 0o666, atimeMs: 0, mtimeMs: 0, ctimeMs: 0 };

	// ---- file descriptors --------------------------------------------------
	let nextFd = 3;
	const fds = new Map(); // fd -> { node, path, pos, append, writable, readable }
	function getFd(fd) {
		const f = fds.get(fd);
		if (!f) throw errnoError("EBADF", "fd " + fd);
		return f;
	}

	// ---- stdout/stderr line buffering -------------------------------------
	const outBufs = { 1: "", 2: "" };
	function emitLine(fd, line) {
		(fd === 2 ? console.error : console.log)(line);
		if (Array.isArray(globalThis.__ikemenBootLog)) {
			globalThis.__ikemenBootLog.push(line);
		}
	}
	function writeStd(fd, buf) {
		outBufs[fd] += dec.decode(buf);
		let nl;
		while ((nl = outBufs[fd].indexOf("\n")) !== -1) {
			emitLine(fd, outBufs[fd].substring(0, nl));
			outBufs[fd] = outBufs[fd].substring(nl + 1);
		}
		return buf.length;
	}

	// Wrap a sync implementation into the Node callback style wasm_exec uses.
	function cbWrap(fn) {
		return function (...args) {
			const callback = args.pop();
			try {
				const res = fn(...args);
				callback(null, res);
			} catch (err) {
				callback(err);
			}
		};
	}

	const constants = {
		O_RDONLY: 0,
		O_WRONLY: 1,
		O_RDWR: 2,
		O_CREAT: 64,
		O_EXCL: 128,
		O_TRUNC: 512,
		O_APPEND: 1024,
		O_DIRECTORY: 65536,
	};

	const fsShim = {
		constants,

		writeSync(fd, buf) {
			if (fd === 1 || fd === 2) return writeStd(fd, buf);
			throw errnoError("EBADF", "writeSync fd " + fd);
		},

		write(fd, buf, offset, length, position, callback) {
			try {
				if (fd === 1 || fd === 2) {
					if (offset !== 0 || length !== buf.length || (position !== null && position !== undefined)) {
						throw errnoError("EINVAL", "stdout write");
					}
					callback(null, writeStd(fd, buf));
					return;
				}
				const f = getFd(fd);
				if (f.node.type !== "file") throw errnoError("EISDIR", f.path);
				if (!f.writable) throw errnoError("EBADF", f.path);
				let pos;
				if (f.append) pos = f.node.size;
				else if (position === null || position === undefined) pos = f.pos;
				else pos = position;
				ensureCapacity(f.node, pos + length);
				if (pos > f.node.size) f.node.buf.fill(0, f.node.size, pos); // sparse gap
				f.node.buf.set(buf.subarray(offset, offset + length), pos);
				f.node.size = Math.max(f.node.size, pos + length);
				f.node.mtimeMs = Date.now();
				if (position === null || position === undefined) f.pos = pos + length;
				persist(f.path, f.node);
				callback(null, length);
			} catch (err) {
				callback(err);
			}
		},

		read(fd, buffer, offset, length, position, callback) {
			try {
				if (fd === 0) { callback(null, 0); return; } // stdin: EOF
				const f = getFd(fd);
				if (f.node.type !== "file") throw errnoError("EISDIR", f.path);
				const pos = (position === null || position === undefined) ? f.pos : position;
				const data = fileData(f.node);
				const n = Math.max(0, Math.min(length, data.length - pos));
				if (n > 0) buffer.set(data.subarray(pos, pos + n), offset);
				if (position === null || position === undefined) f.pos = pos + n;
				callback(null, n);
			} catch (err) {
				callback(err);
			}
		},

		// Not cbWrap'd: opening a lazy node hydrates it first and completes
		// the callback after the awaits (Go's fs_js.go is callback-driven, so
		// late completion is fine). read/write via the fd are then safe.
		open(path, flags, mode, callback) {
			let r;
			try {
				r = lookup(path);
				if (r.notDir) throw errnoError("ENOTDIR", path);
			} catch (err) {
				callback(err);
				return;
			}
			const finish = () => {
				const accmode = flags & 3;
				let node = r.node;
				if (!node) {
					if (!(flags & constants.O_CREAT)) throw errnoError("ENOENT", path);
					if (!r.parent) throw errnoError("ENOENT", path);
					node = makeFile(new Uint8Array(0), mode);
					r.parent.children.set(r.name, node);
					persist(r.realPath, node);
				} else {
					if ((flags & constants.O_CREAT) && (flags & constants.O_EXCL)) throw errnoError("EEXIST", path);
					if ((flags & constants.O_DIRECTORY) && node.type !== "dir") throw errnoError("ENOTDIR", path);
					if (node.type === "dir" && accmode !== constants.O_RDONLY) throw errnoError("EISDIR", path);
					if ((flags & constants.O_TRUNC) && node.type === "file") {
						node.size = 0;
						node.mtimeMs = Date.now();
						persist(r.realPath, node);
					}
				}
				const fd = nextFd++;
				fds.set(fd, {
					node,
					path: r.realPath,
					pos: 0,
					append: !!(flags & constants.O_APPEND),
					readable: accmode === constants.O_RDONLY || accmode === constants.O_RDWR,
					writable: accmode === constants.O_WRONLY || accmode === constants.O_RDWR,
				});
				return fd;
			};
			if (r.node && r.node.type === "file" && r.node.lazy) {
				if (flags & constants.O_TRUNC) {
					discardLazy(r.node); // content is discarded anyway: skip the download
				} else {
					afterHydrate(r.node, path, callback, () => callback(null, finish()));
					return;
				}
			}
			try {
				callback(null, finish());
			} catch (err) {
				callback(err);
			}
		},

		close: cbWrap((fd) => {
			if (!fds.delete(fd) && fd > 2) throw errnoError("EBADF", "fd " + fd);
			return null;
		}),

		fstat: cbWrap((fd) => {
			if (fd >= 0 && fd <= 2) return statFor(ttyNode);
			return statFor(getFd(fd).node);
		}),
		stat: cbWrap((path) => statFor(getNode(path).node)),
		lstat: cbWrap((path) => statFor(getNode(path).node)), // no symlinks

		readdir: cbWrap((path) => {
			const r = getNode(path);
			if (r.node.type !== "dir") throw errnoError("ENOTDIR", path);
			return Array.from(r.node.children.keys());
		}),

		mkdir: cbWrap((path, perm) => {
			const r = lookup(path);
			if (r.notDir) throw errnoError("ENOTDIR", path);
			if (r.node) throw errnoError("EEXIST", path);
			if (!r.parent) throw errnoError("ENOENT", path);
			r.parent.children.set(r.name, makeDir(perm));
			return null;
		}),

		rmdir: cbWrap((path) => {
			const r = getNode(path);
			if (r.node.type !== "dir") throw errnoError("ENOTDIR", path);
			if (r.node.children.size > 0) throw errnoError("ENOTEMPTY", path);
			if (!r.parent) throw errnoError("EBUSY", path);
			r.parent.children.delete(r.name);
			return null;
		}),

		unlink: cbWrap((path) => {
			const r = getNode(path);
			if (r.node.type === "dir") throw errnoError("EISDIR", path);
			r.parent.children.delete(r.name);
			unpersist(r.realPath);
			return null;
		}),

		rename: cbWrap((from, to) => {
			const rf = getNode(from);
			const rt = lookup(to);
			if (rt.notDir) throw errnoError("ENOTDIR", to);
			if (!rt.parent && rt.node !== root) throw errnoError("ENOENT", to);
			if (rt.node && rt.node.type === "dir" && rt.node.children.size > 0) throw errnoError("ENOTEMPTY", to);
			rf.parent.children.delete(rf.name);
			// rt.name is the case-corrected name when the target exists.
			rt.parent.children.set(rt.name, rf.node);
			unpersist(rf.realPath);
			if (rf.node.type === "file") persist(absPath(to), rf.node);
			return null;
		}),

		truncate(path, length, callback) {
			try {
				const r = getNode(path);
				if (r.node.type !== "file") throw errnoError("EISDIR", path);
				const fin = () => {
					truncateNode(r.node, length);
					persist(r.realPath, r.node);
					callback(null, null);
				};
				if (r.node.lazy) {
					if (length === 0) { discardLazy(r.node); fin(); return; }
					afterHydrate(r.node, path, callback, fin); // keeps bytes below length
					return;
				}
				fin();
			} catch (err) {
				callback(err);
			}
		},
		ftruncate: cbWrap((fd, length) => {
			const f = getFd(fd);
			if (f.node.type !== "file") throw errnoError("EISDIR", f.path);
			truncateNode(f.node, length);
			persist(f.path, f.node);
			return null;
		}),

		utimes: cbWrap((path, atime, mtime) => {
			const r = getNode(path);
			r.node.atimeMs = atime * 1000;
			r.node.mtimeMs = mtime * 1000;
			return null;
		}),

		chmod: cbWrap((path, mode) => { getNode(path).node.mode = mode & 0o777; return null; }),
		fchmod: cbWrap((fd, mode) => { getFd(fd).node.mode = mode & 0o777; return null; }),
		chown: cbWrap(() => null),
		fchown: cbWrap(() => null),
		lchown: cbWrap(() => null),
		fsync: cbWrap(() => null),
		readlink: cbWrap((path) => { getNode(path); throw errnoError("EINVAL", path); }),
		link: cbWrap(() => { throw errnoError("ENOSYS"); }),
		symlink: cbWrap(() => { throw errnoError("ENOSYS"); }),
	};

	function truncateNode(node, length) {
		if (length < 0) throw errnoError("EINVAL");
		if (length > node.size) {
			ensureCapacity(node, length);
			node.buf.fill(0, node.size, length);
		}
		node.size = length;
		node.mtimeMs = Date.now();
	}

	const processShim = {
		getuid() { return 0; },
		getgid() { return 0; },
		geteuid() { return 0; },
		getegid() { return 0; },
		getgroups() { return [0]; },
		pid: 1,
		ppid: 0,
		umask() { return 0o22; },
		cwd() { return cwd; },
		chdir(path) {
			// Go stats the path before calling chdir, so errors here are rare;
			// still validate and keep the canonical-cased path.
			const r = lookup(path);
			if (!r.node) throw errnoError("ENOENT", path);
			if (r.node.type !== "dir") throw errnoError("ENOTDIR", path);
			cwd = r.realPath;
		},
	};

	const pathShim = {
		resolve(...segments) {
			let out = "";
			for (const seg of segments) {
				if (seg === "") continue;
				if (String(seg).startsWith("/")) out = String(seg);
				else out = out + "/" + seg;
			}
			if (!out.startsWith("/")) out = cwd + "/" + out;
			return normalize(out);
		},
	};

	// ---- zip mounting -------------------------------------------------------
	async function fetchWithProgress(url, onProgress) {
		const resp = await fetch(url);
		if (!resp.ok) throw new Error("fetch " + url + ": HTTP " + resp.status);
		const total = Number(resp.headers.get("Content-Length")) || 0;
		if (!resp.body) {
			const buf = new Uint8Array(await resp.arrayBuffer());
			if (onProgress) onProgress(buf.length, buf.length);
			return buf;
		}
		const reader = resp.body.getReader();
		const chunks = [];
		let loaded = 0;
		for (;;) {
			const { done, value } = await reader.read();
			if (done) break;
			chunks.push(value);
			loaded += value.length;
			if (onProgress) onProgress(loaded, total);
		}
		const out = new Uint8Array(loaded);
		let off = 0;
		for (const c of chunks) { out.set(c, off); off += c.length; }
		return out;
	}

	async function inflateRaw(bytes) {
		// DecompressionStream is built into Chromium/modern browsers; avoids
		// vendoring an inflate implementation.
		const ds = new DecompressionStream("deflate-raw");
		const stream = new Blob([bytes]).stream().pipeThrough(ds);
		return new Uint8Array(await new Response(stream).arrayBuffer());
	}

	function crc32(bytes) {
		if (!crc32.table) {
			const t = new Uint32Array(256);
			for (let n = 0; n < 256; n++) {
				let c = n;
				for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
				t[n] = c >>> 0;
			}
			crc32.table = t;
		}
		let c = 0xffffffff;
		for (let i = 0; i < bytes.length; i++) c = crc32.table[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
		return (c ^ 0xffffffff) >>> 0;
	}

	// Find the End Of Central Directory record (sig 0x06054b50), scanning
	// backwards past a possible zip comment (max 64k). bytes may be just the
	// tail of the archive; cdOffset/cdSize are archive-absolute either way.
	function findEOCD(bytes) {
		const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
		let eocd = -1;
		const scanStart = Math.max(0, bytes.length - 65557);
		for (let i = bytes.length - 22; i >= scanStart; i--) {
			if (view.getUint32(i, true) === 0x06054b50) { eocd = i; break; }
		}
		if (eocd < 0) throw new Error("zip: end of central directory not found");
		const count = view.getUint16(eocd + 10, true);
		const cdSize = view.getUint32(eocd + 12, true);
		const cdOffset = view.getUint32(eocd + 16, true);
		if (cdOffset === 0xffffffff || count === 0xffff) throw new Error("zip: zip64 archives not supported");
		return { count, cdSize, cdOffset };
	}

	// Parse count central-directory entries from cd, a buffer that starts at
	// the first entry (sizes there are valid even when entries use streaming
	// data descriptors).
	function parseCentralDirectory(cd, count) {
		const view = new DataView(cd.buffer, cd.byteOffset, cd.byteLength);
		const entries = [];
		let off = 0;
		for (let i = 0; i < count; i++) {
			if (view.getUint32(off, true) !== 0x02014b50) throw new Error("zip: bad central directory entry");
			const method = view.getUint16(off + 10, true);
			const crc = view.getUint32(off + 16, true);
			const compSize = view.getUint32(off + 20, true);
			const size = view.getUint32(off + 24, true);
			const nameLen = view.getUint16(off + 28, true);
			const extraLen = view.getUint16(off + 30, true);
			const commentLen = view.getUint16(off + 32, true);
			const localOff = view.getUint32(off + 42, true);
			if (compSize === 0xffffffff || size === 0xffffffff || localOff === 0xffffffff) throw new Error("zip: zip64 entries not supported");
			entries.push({ name: dec.decode(cd.subarray(off + 46, off + 46 + nameLen)), method, crc, compSize, size, localOff });
			off += 46 + nameLen + extraLen + commentLen;
		}
		return entries;
	}

	// Minimal zip extractor driven by the central directory.
	// opts (optional): { mapName(name) -> newName|null to skip,
	//                    onProgress(entriesDone, entriesTotal) }
	async function extractZip(bytes, mountPoint, opts) {
		opts = opts || {};
		const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
		const eocd = findEOCD(bytes);
		const entries = parseCentralDirectory(bytes.subarray(eocd.cdOffset), eocd.count);
		let files = 0;
		for (let i = 0; i < entries.length; i++) {
			const e = entries[i];
			if (opts.onProgress) opts.onProgress(i + 1, entries.length);
			let name = e.name;
			if (opts.mapName) name = opts.mapName(name);
			if (!name) continue; // skipped by mapName
			if (name.includes("..")) continue; // hostile path, skip
			const target = mountPoint + "/" + name;
			if (name.endsWith("/")) {
				mkdirAt(target);
				continue;
			}
			// Local header repeats name/extra with possibly different extra len.
			if (view.getUint32(e.localOff, true) !== 0x04034b50) throw new Error("zip: bad local header for " + name);
			const lNameLen = view.getUint16(e.localOff + 26, true);
			const lExtraLen = view.getUint16(e.localOff + 28, true);
			const dataStart = e.localOff + 30 + lNameLen + lExtraLen;
			const raw = bytes.subarray(dataStart, dataStart + e.compSize);
			let data;
			if (e.method === 0) data = raw.slice();
			else if (e.method === 8) data = await inflateRaw(raw);
			else throw new Error("zip: unsupported compression method " + e.method + " for " + name);
			writeFileAt(target, data, false);
			files++;
		}
		return files;
	}

	async function mountZip(url, mountPoint, onProgress) {
		mountPoint = absPath(mountPoint);
		mkdirAt(mountPoint);
		const bytes = await fetchWithProgress(url, onProgress);
		const files = await extractZip(bytes, mountPoint);
		const restored = restoreSaves();
		return { files, restored, bytes: bytes.length };
	}

	// Mount from an in-memory zip (ArrayBuffer or Uint8Array), e.g. a
	// user-supplied file. opts may be a progress callback
	// (entriesDone, entriesTotal) or the extractZip options object.
	async function mountZipBuffer(buf, mountPoint, opts) {
		if (typeof opts === "function") opts = { onProgress: opts };
		mountPoint = absPath(mountPoint);
		mkdirAt(mountPoint);
		const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
		const files = await extractZip(bytes, mountPoint, opts);
		return { files, bytes: bytes.length };
	}

	// ---- lazy zip mounting ---------------------------------------------------
	// mountZipIndex fetches only the central directory and registers every
	// entry as a lazy node; hydrate() Range-fetches and inflates a file's
	// bytes on first open. Falls back to mountZip when the host lacks HTTP
	// Range support.
	const INDEX_TAIL = 128 * 1024;
	const LOCAL_HDR_SLACK = 30 + 1024; // local header + name/extra headroom

	const lazyState = { files: 0, hydrated: 0, bytes: 0, hydratedBytes: 0 };
	globalThis.__fsLazyStats = lazyState;

	async function fetchRange(url, start, end, ifRange) {
		const headers = { "Range": "bytes=" + start + "-" + end };
		if (ifRange) headers["If-Range"] = ifRange;
		for (let attempt = 0; ; attempt++) {
			try {
				const resp = await fetch(url, { headers });
				if (!resp.ok) throw new Error("fetch " + url + ": HTTP " + resp.status);
				return { buf: new Uint8Array(await resp.arrayBuffer()), partial: resp.status === 206 };
			} catch (err) {
				// Retry once: a transient network failure would otherwise
				// surface to the engine as EIO on open.
				if (attempt >= 1) throw err;
			}
		}
	}

	// One in-flight hydration promise per node; concurrent opens share it.
	function hydrate(node, path) {
		if (!node.lazy) return Promise.resolve();
		if (node.lazyFetch) return node.lazyFetch;
		const l = node.lazy;
		node.lazyFetch = (async () => {
			let data;
			if (l.compSize === 0 && l.size === 0) {
				data = new Uint8Array(0); // empty entry: no request needed
			} else {
				// One Range request with slack for the local header's
				// name/extra fields (their lengths can differ from the
				// central directory's); a second request fetches any
				// missing tail if the slack was insufficient.
				const first = await fetchRange(l.url, l.hdrOffset, l.hdrOffset + LOCAL_HDR_SLACK + l.compSize - 1, l.ifRange);
				// 200 means the host ignored Range (or If-Range detected a
				// changed archive): the body is the whole file.
				const off0 = first.partial ? l.hdrOffset : 0;
				const buf = first.buf;
				const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
				const hdr = l.hdrOffset - off0;
				if (buf.length < hdr + 30 || view.getUint32(hdr, true) !== 0x04034b50) {
					throw new Error("zip: bad local header for " + path + " (changed archive?)");
				}
				const dataStart = hdr + 30 + view.getUint16(hdr + 26, true) + view.getUint16(hdr + 28, true);
				let raw = buf.subarray(dataStart, Math.min(dataStart + l.compSize, buf.length));
				if (raw.length < l.compSize) {
					// First missing data byte is at dataStart + raw.length
					// (past the end of buf, or past a giant extra field).
					const rest = await fetchRange(l.url, off0 + dataStart + raw.length, off0 + dataStart + l.compSize - 1, l.ifRange);
					if (!rest.partial) throw new Error("zip: archive changed mid-hydration for " + path);
					if (raw.length + rest.buf.length < l.compSize) throw new Error("zip: short range response for " + path);
					const joined = new Uint8Array(l.compSize);
					joined.set(raw, 0);
					joined.set(rest.buf.subarray(0, l.compSize - raw.length), raw.length);
					raw = joined;
				}
				if (l.method === 0) data = raw.slice();
				else if (l.method === 8) data = await inflateRaw(raw);
				else throw new Error("zip: unsupported compression method " + l.method + " for " + path);
				if (data.length !== l.size || crc32(data) !== l.crc) {
					throw new Error("zip: checksum mismatch for " + path + " (stale cache or changed archive?)");
				}
			}
			if (node.lazy === l) { // node not truncated/replaced meanwhile
				node.buf = data;
				node.size = data.length;
				delete node.lazy;
				lazyState.hydrated++;
				lazyState.hydratedBytes += l.compSize;
			}
		})();
		node.lazyFetch.then(() => { delete node.lazyFetch; }, () => { delete node.lazyFetch; });
		return node.lazyFetch;
	}

	// O_TRUNC / truncate(0) on a lazy node discards the content unread: no
	// download. An in-flight hydrate() sees node.lazy gone and drops its data.
	function discardLazy(node) {
		node.buf = new Uint8Array(0);
		node.size = 0;
		delete node.lazy;
		lazyState.hydrated++;
	}

	// Run fn once node's content is hydrated; failures reach the callback
	// with an errno code Go can map.
	function afterHydrate(node, path, callback, fn) {
		hydrate(node, path).then(() => {
			try { fn(); } catch (err) { callback(err); }
		}, (err) => {
			if (!err.code) err.code = "EIO";
			callback(err);
		});
	}

	// Probe Range support with a suffix request for the archive tail (a host
	// without it answers 200 + full body, which we abandon), then locate the
	// EOCD and fetch/parse the central directory.
	async function fetchZipIndex(url) {
		const resp = await fetch(url, { headers: { "Range": "bytes=-" + INDEX_TAIL } });
		if (!resp.ok) throw new Error("fetch " + url + ": HTTP " + resp.status);
		if (resp.status !== 206) {
			try { if (resp.body) await resp.body.cancel(); } catch (e) { /* ignore */ }
			throw new Error("no HTTP Range support (got " + resp.status + ")");
		}
		const m = /bytes (\d+)-(\d+)\/(\d+)/.exec(resp.headers.get("Content-Range") || "");
		if (!m) throw new Error("unparseable Content-Range: " + resp.headers.get("Content-Range"));
		const total = Number(m[3]);
		const tail = new Uint8Array(await resp.arrayBuffer());
		const tailStart = total - tail.length;
		// Validator for later Range requests: a redeploy mid-session makes
		// If-Range return the full (new) body instead of stale-offset bytes.
		const ifRange = resp.headers.get("ETag") || resp.headers.get("Last-Modified") || null;
		const eocd = findEOCD(tail);
		let cd;
		if (eocd.cdOffset >= tailStart) {
			cd = tail.subarray(eocd.cdOffset - tailStart, eocd.cdOffset - tailStart + eocd.cdSize);
		} else {
			const r = await fetchRange(url, eocd.cdOffset, eocd.cdOffset + eocd.cdSize - 1, ifRange);
			cd = r.partial ? r.buf : r.buf.subarray(eocd.cdOffset, eocd.cdOffset + eocd.cdSize);
		}
		return { entries: parseCentralDirectory(cd, eocd.count), total, ifRange };
	}

	// Lazy mount: returns { lazy: true, files, restored, total } on success;
	// on any index/Range failure falls back to the full-download mountZip and
	// returns its result plus { lazy: false }. opts: { onProgress } (used only
	// by the fallback download).
	async function mountZipIndex(url, mountPoint, opts) {
		opts = opts || {};
		mountPoint = absPath(mountPoint);
		let idx;
		try {
			idx = await fetchZipIndex(url);
		} catch (err) {
			console.warn("fs-shim: lazy mount of " + url + " unavailable (" + ((err && err.message) || err) + "); downloading in full");
			const res = await mountZip(url, mountPoint, opts.onProgress);
			res.lazy = false;
			return res;
		}
		mkdirAt(mountPoint);
		let files = 0;
		for (const e of idx.entries) {
			if (e.name.includes("..")) continue; // hostile path, skip
			const target = mountPoint + "/" + e.name;
			if (e.name.endsWith("/")) {
				mkdirAt(target);
				continue;
			}
			setFileAt(target, makeLazyFile({
				url, hdrOffset: e.localOff, compSize: e.compSize, size: e.size,
				method: e.method, crc: e.crc, ifRange: idx.ifRange,
			}));
			lazyState.files++;
			lazyState.bytes += e.compSize;
			files++;
		}
		const restored = restoreSaves(); // /save/ mirror wins over lazy nodes
		return { lazy: true, files, restored, total: idx.total };
	}

	// Hydrate every still-lazy file under mountPoint whose relative path
	// starts with one of prefixes (case-insensitive), a few in parallel.
	// onProgress(doneBytes, totalBytes) counts compressed bytes.
	async function prefetchLazy(mountPoint, prefixes, onProgress) {
		mountPoint = absPath(mountPoint);
		const lows = prefixes.map((p) => p.toLowerCase());
		const queue = [];
		(function walk(dir, rel) {
			for (const [name, node] of dir.children) {
				if (node.type === "dir") walk(node, rel + name + "/");
				else if (node.lazy && lows.some((p) => (rel + name).toLowerCase().startsWith(p))) {
					queue.push({ path: mountPoint + "/" + rel + name, node, n: node.lazy.compSize });
				}
			}
		})(getNode(mountPoint).node, "");
		const total = queue.reduce((s, t) => s + t.n, 0);
		const count = queue.length;
		let done = 0;
		const workers = [];
		for (let i = 0; i < 6 && i < queue.length; i++) {
			workers.push((async () => {
				for (let t; (t = queue.shift()) !== undefined; ) {
					await hydrate(t.node, t.path);
					done += t.n;
					if (onProgress) onProgress(done, total);
				}
			})());
		}
		await Promise.all(workers);
		return { files: count, bytes: total };
	}

	// Overwrite unconditionally: wasm_exec.js may have installed its ENOSYS
	// stubs already (script order is not guaranteed), and it resolves
	// globalThis.fs lazily at call time.
	globalThis.fs = fsShim;
	globalThis.process = processShim;
	globalThis.path = pathShim;
	globalThis.mountZip = mountZip;
	globalThis.mountZipBuffer = mountZipBuffer;
	globalThis.mountZipIndex = mountZipIndex;
	globalThis.prefetchLazy = prefetchLazy;
})();
