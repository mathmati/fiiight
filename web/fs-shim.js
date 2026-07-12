// fs-shim.js -- in-memory POSIX-ish filesystem for Go js/wasm (wasm_exec.js).
// Provides globalThis.fs / globalThis.process / globalThis.path with the
// Node-style callback API that Go's syscall/fs_js.go calls, plus
// mountZip(url, mountPoint, onProgress) to seed the tree from a zip archive.
// Files under a "/save/" path segment are mirrored to localStorage and
// restored at mount time.
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
	function fileData(node) {
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
		if (!isSavePath(path) || node.type !== "file") return;
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

	// mkdir -p + write, used by zip mount and save restore.
	function writeFileAt(path, bytes, doPersist) {
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
		const node = makeFile(bytes);
		dir.children.set(hit ? hit.name : name, node);
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

		open: cbWrap((path, flags, mode) => {
			const accmode = flags & 3;
			const r = lookup(path);
			if (r.notDir) throw errnoError("ENOTDIR", path);
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
		}),

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

		truncate: cbWrap((path, length) => {
			const r = getNode(path);
			if (r.node.type !== "file") throw errnoError("EISDIR", path);
			truncateNode(r.node, length);
			persist(r.realPath, r.node);
			return null;
		}),
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

	// Minimal zip reader driven by the central directory (sizes there are
	// valid even when entries use streaming data descriptors).
	async function extractZip(bytes, mountPoint) {
		const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
		// Find End Of Central Directory record (sig 0x06054b50), scanning
		// backwards past a possible zip comment (max 64k).
		let eocd = -1;
		const scanStart = Math.max(0, bytes.length - 65557);
		for (let i = bytes.length - 22; i >= scanStart; i--) {
			if (view.getUint32(i, true) === 0x06054b50) { eocd = i; break; }
		}
		if (eocd < 0) throw new Error("zip: end of central directory not found");
		const count = view.getUint16(eocd + 10, true);
		let off = view.getUint32(eocd + 16, true);
		if (off === 0xffffffff || count === 0xffff) throw new Error("zip: zip64 archives not supported");
		let files = 0;
		for (let i = 0; i < count; i++) {
			if (view.getUint32(off, true) !== 0x02014b50) throw new Error("zip: bad central directory entry");
			const method = view.getUint16(off + 10, true);
			const compSize = view.getUint32(off + 20, true);
			const nameLen = view.getUint16(off + 28, true);
			const extraLen = view.getUint16(off + 30, true);
			const commentLen = view.getUint16(off + 32, true);
			const localOff = view.getUint32(off + 42, true);
			const name = dec.decode(bytes.subarray(off + 46, off + 46 + nameLen));
			off += 46 + nameLen + extraLen + commentLen;

			if (name.includes("..")) continue; // hostile path, skip
			const target = mountPoint + "/" + name;
			if (name.endsWith("/")) {
				mkdirAt(target);
				continue;
			}
			// Local header repeats name/extra with possibly different extra len.
			if (view.getUint32(localOff, true) !== 0x04034b50) throw new Error("zip: bad local header for " + name);
			const lNameLen = view.getUint16(localOff + 26, true);
			const lExtraLen = view.getUint16(localOff + 28, true);
			const dataStart = localOff + 30 + lNameLen + lExtraLen;
			const raw = bytes.subarray(dataStart, dataStart + compSize);
			let data;
			if (method === 0) data = raw.slice();
			else if (method === 8) data = await inflateRaw(raw);
			else throw new Error("zip: unsupported compression method " + method + " for " + name);
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

	// Overwrite unconditionally: wasm_exec.js may have installed its ENOSYS
	// stubs already (script order is not guaranteed), and it resolves
	// globalThis.fs lazily at call time.
	globalThis.fs = fsShim;
	globalThis.process = processShim;
	globalThis.path = pathShim;
	globalThis.mountZip = mountZip;
})();
