//go:build js

// Browser (js/wasm) input backend: keyboard via KeyboardEvent.code,
// controllers via the browser Gamepad API (standard mapping).
package main

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"regexp"
	"strings"
	"syscall/js"
)

// Key is the backend keycode type (browser build).
type Key int32

// ModifierKey is a bitmask of modifier keys (browser build).
type ModifierKey int32

// Backend-provided modifier key masks (see ShortcutKey.Test in input.go)
const (
	KModShift ModifierKey = 1 << iota
	KModCtrl
	KModAlt
	KModGui
)

// Keycodes. The numeric values are private to the js backend; only the
// string names in KeyToStringLUT are persisted in config files.
const (
	KeyUnknown Key = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeySpace
	KeyQuote
	KeyComma
	KeyMinus
	KeyPeriod
	KeySlash
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeySemicolon
	KeyEquals
	KeyLBracket
	KeyBackslash
	KeyRBracket
	KeyBackquote
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	KeyCapsLock
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyPrintScreen
	KeyScrollLock
	KeyPause
	KeyInsert
	KeyHome
	KeyPageUp
	KeyDelete
	KeyEnd
	KeyPageDown
	KeyRight
	KeyLeft
	KeyDown
	KeyUp
	KeyNumLockClear
	KeyKPDivide
	KeyKPMultiply
	KeyKPMinus
	KeyKPPlus
	KeyKPEnter
	KeyKP1
	KeyKP2
	KeyKP3
	KeyKP4
	KeyKP5
	KeyKP6
	KeyKP7
	KeyKP8
	KeyKP9
	KeyKP0
	KeyKPPeriod
	KeyKPEquals
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
	KeyMenu
	KeyLCtrl
	KeyLShift
	KeyLAlt
	KeyLGui
	KeyRCtrl
	KeyRShift
	KeyRAlt
	KeyRGui
)

var KeyToStringLUT map[Key]string
var StringToKeyLUT map[string]Key
var ButtonToStringLUT map[int]string
var buttonOrder []int
var StringToButtonLUT map[string]int

// jsCodeToKeyLUT maps KeyboardEvent.code values to backend keycodes.
var jsCodeToKeyLUT map[string]Key

type ControllerState struct {
	Axes      [6]int8
	Buttons   map[int]byte
	HasRumble bool
}

type Input struct {
	// controllers holds browser Gamepad indices (navigator.getGamepads()
	// slot numbers); -1 means no controller attached to this player slot.
	controllers     [MaxPlayerNo]int
	controllerstate [MaxPlayerNo]*ControllerState
}

var input Input

// SDL GameController button order, reproduced for the js backend.
const (
	jsButtonA = iota
	jsButtonB
	jsButtonX
	jsButtonY
	jsButtonBack
	jsButtonGuide
	jsButtonStart
	jsButtonLeftStick
	jsButtonRightStick
	jsButtonLeftShoulder
	jsButtonRightShoulder
	jsButtonDpadUp
	jsButtonDpadDown
	jsButtonDpadLeft
	jsButtonDpadRight
)

func initLUTs() {
	KeyToStringLUT = map[Key]string{
		KeyEnter:        "RETURN",
		KeyEscape:       "ESCAPE",
		KeyBackspace:    "BACKSPACE",
		KeyTab:          "TAB",
		KeySpace:        "SPACE",
		KeyQuote:        "QUOTE",
		KeyComma:        "COMMA",
		KeyMinus:        "MINUS",
		KeyPeriod:       "PERIOD",
		KeySlash:        "SLASH",
		Key0:            "0",
		Key1:            "1",
		Key2:            "2",
		Key3:            "3",
		Key4:            "4",
		Key5:            "5",
		Key6:            "6",
		Key7:            "7",
		Key8:            "8",
		Key9:            "9",
		KeySemicolon:    "SEMICOLON",
		KeyEquals:       "EQUALS",
		KeyLBracket:     "LBRACKET",
		KeyBackslash:    "BACKSLASH",
		KeyRBracket:     "RBRACKET",
		KeyBackquote:    "BACKQUOTE",
		KeyA:            "a",
		KeyB:            "b",
		KeyC:            "c",
		KeyD:            "d",
		KeyE:            "e",
		KeyF:            "f",
		KeyG:            "g",
		KeyH:            "h",
		KeyI:            "i",
		KeyJ:            "j",
		KeyK:            "k",
		KeyL:            "l",
		KeyM:            "m",
		KeyN:            "n",
		KeyO:            "o",
		KeyP:            "p",
		KeyQ:            "q",
		KeyR:            "r",
		KeyS:            "s",
		KeyT:            "t",
		KeyU:            "u",
		KeyV:            "v",
		KeyW:            "w",
		KeyX:            "x",
		KeyY:            "y",
		KeyZ:            "z",
		KeyCapsLock:     "CAPSLOCK",
		KeyF1:           "F1",
		KeyF2:           "F2",
		KeyF3:           "F3",
		KeyF4:           "F4",
		KeyF5:           "F5",
		KeyF6:           "F6",
		KeyF7:           "F7",
		KeyF8:           "F8",
		KeyF9:           "F9",
		KeyF10:          "F10",
		KeyF11:          "F11",
		KeyF12:          "F12",
		KeyPrintScreen:  "PRINTSCREEN",
		KeyScrollLock:   "SCROLLLOCK",
		KeyPause:        "PAUSE",
		KeyInsert:       "INSERT",
		KeyHome:         "HOME",
		KeyPageUp:       "PAGEUP",
		KeyDelete:       "DELETE",
		KeyEnd:          "END",
		KeyPageDown:     "PAGEDOWN",
		KeyRight:        "RIGHT",
		KeyLeft:         "LEFT",
		KeyDown:         "DOWN",
		KeyUp:           "UP",
		KeyNumLockClear: "NUMLOCKCLEAR",
		KeyKPDivide:     "KP_DIVIDE",
		KeyKPMultiply:   "KP_MULTIPLY",
		KeyKPMinus:      "KP_MINUS",
		KeyKPPlus:       "KP_PLUS",
		KeyKPEnter:      "KP_ENTER",
		KeyKP1:          "KP_1",
		KeyKP2:          "KP_2",
		KeyKP3:          "KP_3",
		KeyKP4:          "KP_4",
		KeyKP5:          "KP_5",
		KeyKP6:          "KP_6",
		KeyKP7:          "KP_7",
		KeyKP8:          "KP_8",
		KeyKP9:          "KP_9",
		KeyKP0:          "KP_0",
		KeyKPPeriod:     "KP_PERIOD",
		KeyKPEquals:     "KP_EQUALS",
		KeyF13:          "F13",
		KeyF14:          "F14",
		KeyF15:          "F15",
		KeyF16:          "F16",
		KeyF17:          "F17",
		KeyF18:          "F18",
		KeyF19:          "F19",
		KeyF20:          "F20",
		KeyF21:          "F21",
		KeyF22:          "F22",
		KeyF23:          "F23",
		KeyF24:          "F24",
		KeyMenu:         "MENU",
		KeyLCtrl:        "LCTRL",
		KeyLShift:       "LSHIFT",
		KeyLAlt:         "LALT",
		KeyLGui:         "LGUI",
		KeyRCtrl:        "RCTRL",
		KeyRShift:       "RSHIFT",
		KeyRAlt:         "RALT",
		KeyRGui:         "RGUI",
	}

	buttonOrder = []int{
		jsButtonA,
		jsButtonB,
		jsButtonX,
		jsButtonY,
		jsButtonBack,
		jsButtonGuide,
		jsButtonStart,
		jsButtonLeftStick,
		jsButtonRightStick,
		jsButtonLeftShoulder,
		jsButtonRightShoulder,
		jsButtonDpadUp,
		jsButtonDpadDown,
		jsButtonDpadLeft,
		jsButtonDpadRight,
	}

	ButtonToStringLUT = map[int]string{
		jsButtonA:             "A",
		jsButtonB:             "B",
		jsButtonX:             "X",
		jsButtonY:             "Y",
		jsButtonBack:          "BACK",
		jsButtonGuide:         "HOME",
		jsButtonStart:         "START",
		jsButtonLeftStick:     "LS",
		jsButtonRightStick:    "RS",
		jsButtonLeftShoulder:  "LB",
		jsButtonRightShoulder: "RB",
		jsButtonDpadUp:        "DP_U",
		jsButtonDpadDown:      "DP_D",
		jsButtonDpadLeft:      "DP_L",
		jsButtonDpadRight:     "DP_R",
		15:                    "LS_Y-",
		16:                    "LS_X-",
		17:                    "LS_X+",
		18:                    "LS_Y+",
		19:                    "LT",
		20:                    "RT",
		21:                    "RS_Y-",
		22:                    "RS_X-",
		23:                    "RS_X+",
		24:                    "RS_Y+",
		25:                    "Not used",
	}

	// Explicitly allocate the maps here
	StringToKeyLUT = make(map[string]Key)
	StringToButtonLUT = make(map[string]int)

	for k, v := range KeyToStringLUT {
		StringToKeyLUT[v] = k
	}
	for k, v := range ButtonToStringLUT {
		StringToButtonLUT[v] = k
	}

	// KeyboardEvent.code -> Key
	jsCodeToKeyLUT = map[string]Key{
		"Enter":          KeyEnter,
		"Escape":         KeyEscape,
		"Backspace":      KeyBackspace,
		"Tab":            KeyTab,
		"Space":          KeySpace,
		"Quote":          KeyQuote,
		"Comma":          KeyComma,
		"Minus":          KeyMinus,
		"Period":         KeyPeriod,
		"Slash":          KeySlash,
		"Digit0":         Key0,
		"Digit1":         Key1,
		"Digit2":         Key2,
		"Digit3":         Key3,
		"Digit4":         Key4,
		"Digit5":         Key5,
		"Digit6":         Key6,
		"Digit7":         Key7,
		"Digit8":         Key8,
		"Digit9":         Key9,
		"Semicolon":      KeySemicolon,
		"Equal":          KeyEquals,
		"BracketLeft":    KeyLBracket,
		"Backslash":      KeyBackslash,
		"BracketRight":   KeyRBracket,
		"Backquote":      KeyBackquote,
		"KeyA":           KeyA,
		"KeyB":           KeyB,
		"KeyC":           KeyC,
		"KeyD":           KeyD,
		"KeyE":           KeyE,
		"KeyF":           KeyF,
		"KeyG":           KeyG,
		"KeyH":           KeyH,
		"KeyI":           KeyI,
		"KeyJ":           KeyJ,
		"KeyK":           KeyK,
		"KeyL":           KeyL,
		"KeyM":           KeyM,
		"KeyN":           KeyN,
		"KeyO":           KeyO,
		"KeyP":           KeyP,
		"KeyQ":           KeyQ,
		"KeyR":           KeyR,
		"KeyS":           KeyS,
		"KeyT":           KeyT,
		"KeyU":           KeyU,
		"KeyV":           KeyV,
		"KeyW":           KeyW,
		"KeyX":           KeyX,
		"KeyY":           KeyY,
		"KeyZ":           KeyZ,
		"CapsLock":       KeyCapsLock,
		"F1":             KeyF1,
		"F2":             KeyF2,
		"F3":             KeyF3,
		"F4":             KeyF4,
		"F5":             KeyF5,
		"F6":             KeyF6,
		"F7":             KeyF7,
		"F8":             KeyF8,
		"F9":             KeyF9,
		"F10":            KeyF10,
		"F11":            KeyF11,
		"F12":            KeyF12,
		"F13":            KeyF13,
		"F14":            KeyF14,
		"F15":            KeyF15,
		"F16":            KeyF16,
		"F17":            KeyF17,
		"F18":            KeyF18,
		"F19":            KeyF19,
		"F20":            KeyF20,
		"F21":            KeyF21,
		"F22":            KeyF22,
		"F23":            KeyF23,
		"F24":            KeyF24,
		"PrintScreen":    KeyPrintScreen,
		"ScrollLock":     KeyScrollLock,
		"Pause":          KeyPause,
		"Insert":         KeyInsert,
		"Home":           KeyHome,
		"PageUp":         KeyPageUp,
		"Delete":         KeyDelete,
		"End":            KeyEnd,
		"PageDown":       KeyPageDown,
		"ArrowRight":     KeyRight,
		"ArrowLeft":      KeyLeft,
		"ArrowDown":      KeyDown,
		"ArrowUp":        KeyUp,
		"NumLock":        KeyNumLockClear,
		"NumpadDivide":   KeyKPDivide,
		"NumpadMultiply": KeyKPMultiply,
		"NumpadSubtract": KeyKPMinus,
		"NumpadAdd":      KeyKPPlus,
		"NumpadEnter":    KeyKPEnter,
		"Numpad1":        KeyKP1,
		"Numpad2":        KeyKP2,
		"Numpad3":        KeyKP3,
		"Numpad4":        KeyKP4,
		"Numpad5":        KeyKP5,
		"Numpad6":        KeyKP6,
		"Numpad7":        KeyKP7,
		"Numpad8":        KeyKP8,
		"Numpad9":        KeyKP9,
		"Numpad0":        KeyKP0,
		"NumpadDecimal":  KeyKPPeriod,
		"NumpadEqual":    KeyKPEquals,
		"ContextMenu":    KeyMenu,
		"ControlLeft":    KeyLCtrl,
		"ShiftLeft":      KeyLShift,
		"AltLeft":        KeyLAlt,
		"MetaLeft":       KeyLGui,
		"ControlRight":   KeyRCtrl,
		"ShiftRight":     KeyRShift,
		"AltRight":       KeyRAlt,
		"MetaRight":      KeyRGui,
	}

	input = Input{}
	for i := range input.controllers {
		input.controllers[i] = -1
	}
}

// jsCodeToKey converts a KeyboardEvent.code string to a backend keycode.
func jsCodeToKey(code string) Key {
	if k, ok := jsCodeToKeyLUT[code]; ok {
		return k
	}
	return KeyUnknown
}

// modifierFromEvent builds a ModifierKey mask from a KeyboardEvent.
func modifierFromEvent(e js.Value) (mod ModifierKey) {
	if e.Get("ctrlKey").Truthy() {
		mod |= KModCtrl
	}
	if e.Get("altKey").Truthy() {
		mod |= KModAlt
	}
	if e.Get("shiftKey").Truthy() {
		mod |= KModShift
	}
	if e.Get("metaKey").Truthy() {
		mod |= KModGui
	}
	return
}

func StringToKey(s string) Key {
	if key, ok := StringToKeyLUT[s]; ok {
		return key
	}
	return KeyUnknown
}

func KeyToString(k Key) string {
	if s, ok := KeyToStringLUT[k]; ok {
		return s
	}
	return ""
}

func NewModifierKey(ctrl, alt, shift bool) (mod ModifierKey) {
	if ctrl {
		mod |= KModCtrl
	}
	if alt {
		mod |= KModAlt
	}
	if shift {
		mod |= KModShift
	}
	return
}

// getGamepads returns the navigator.getGamepads() array, or the zero value
// when the Gamepad API is unavailable.
func getGamepads() js.Value {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() || !navigator.Get("getGamepads").Truthy() {
		return js.Value{}
	}
	return navigator.Call("getGamepads")
}

// getGamepad returns the live Gamepad object for a player slot, or the zero
// value if none is attached.
func getGamepad(joy int) js.Value {
	if joy < 0 || joy >= len(input.controllers) || input.controllers[joy] < 0 {
		return js.Value{}
	}
	pads := getGamepads()
	if !pads.Truthy() || input.controllers[joy] >= pads.Length() {
		return js.Value{}
	}
	pad := pads.Index(input.controllers[joy])
	if !pad.Truthy() {
		return js.Value{}
	}
	return pad
}

func axisFloatToI8(v float64) int8 {
	if v < -1 {
		v = -1
	} else if v > 1 {
		v = 1
	}
	if v < 0 {
		return int8(v * 128)
	}
	return int8(v * 127)
}

// pollGamepads snapshots the browser gamepads into input.controllerstate.
// Called every frame from Window.pollEvents.
func pollGamepads() {
	pads := getGamepads()
	if !pads.Truthy() {
		return
	}

	// Attach newly connected pads / detach removed ones
	seen := make(map[int]bool)
	for i := 0; i < pads.Length(); i++ {
		pad := pads.Index(i)
		if !pad.Truthy() || !pad.Get("connected").Truthy() {
			continue
		}
		seen[i] = true
		// Already attached?
		attached := false
		for slot := range input.controllers {
			if input.controllers[slot] == i {
				attached = true
				break
			}
		}
		if !attached {
			for slot := range input.controllers {
				if input.controllers[slot] < 0 {
					input.controllers[slot] = i
					input.controllerstate[slot] = &ControllerState{Buttons: make(map[int]byte)}
					input.controllerstate[slot].HasRumble = pad.Get("vibrationActuator").Truthy()
					break
				}
			}
		}
	}
	for slot := range input.controllers {
		if input.controllers[slot] >= 0 && !seen[input.controllers[slot]] {
			input.controllers[slot] = -1
			if input.controllerstate[slot] != nil {
				input.controllerstate[slot].Axes = [6]int8{}
				for k := range input.controllerstate[slot].Buttons {
					delete(input.controllerstate[slot].Buttons, k)
				}
				input.controllerstate[slot].HasRumble = false
			}
		}
	}

	// Standard-mapping browser button index -> SDL-style button index
	stdToButton := [17]int{
		jsButtonA, jsButtonB, jsButtonX, jsButtonY,
		jsButtonLeftShoulder, jsButtonRightShoulder,
		-1, -1, // 6/7 = triggers, handled as axes below
		jsButtonBack, jsButtonStart,
		jsButtonLeftStick, jsButtonRightStick,
		jsButtonDpadUp, jsButtonDpadDown, jsButtonDpadLeft, jsButtonDpadRight,
		jsButtonGuide,
	}

	for slot := range input.controllers {
		if input.controllers[slot] < 0 || input.controllerstate[slot] == nil {
			continue
		}
		pad := pads.Index(input.controllers[slot])
		if !pad.Truthy() {
			continue
		}
		state := input.controllerstate[slot]

		axes := pad.Get("axes")
		for a := 0; a < 4 && a < axes.Length(); a++ {
			state.Axes[a] = axisFloatToI8(axes.Index(a).Float())
		}

		buttons := pad.Get("buttons")
		for b := 0; b < buttons.Length() && b < len(stdToButton); b++ {
			btn := buttons.Index(b)
			pressed := btn.Get("pressed").Truthy()
			switch b {
			case 6: // left trigger
				state.Axes[4] = axisFloatToI8(btn.Get("value").Float())
			case 7: // right trigger
				state.Axes[5] = axisFloatToI8(btn.Get("value").Float())
			default:
				if idx := stdToButton[b]; idx >= 0 {
					if pressed {
						state.Buttons[idx] = 1
					} else {
						state.Buttons[idx] = 0
					}
				}
			}
		}
	}
}

// UpdateGamepadMappings is a no-op on js; the browser exposes gamepads with
// the standard mapping already applied.
func (input *Input) UpdateGamepadMappings(path string) {}

func (input *Input) GetMaxJoystickCount() int {
	return len(input.controllers)
}

func (input *Input) IsJoystickPresent(joy int) bool {
	if joy < 0 || joy >= len(input.controllers) {
		return false
	}
	return getGamepad(joy).Truthy()
}

func (input *Input) GetJoystickName(joy int) string {
	pad := getGamepad(joy)
	if !pad.Truthy() {
		return ""
	}
	return pad.Get("id").String()
}

func (input *Input) GetJoystickAxes(joy int) [6]float32 {
	if joy < 0 || joy >= len(input.controllerstate) {
		return [6]float32{0, 0, 0, 0, 0, 0}
	}
	if input.controllerstate[joy] == nil {
		return [6]float32{0, 0, 0, 0, 0, 0}
	}
	axes := NormalizeAxes(&input.controllerstate[joy].Axes)
	return axes
}

func (input *Input) GetJoystickButtons(joy int) []byte {
	if joy < 0 || joy >= len(input.controllerstate) {
		return []byte{}
	}
	if input.controllerstate[joy] == nil {
		return []byte{}
	}
	buttons := make([]byte, len(buttonOrder))
	for i, button := range buttonOrder {
		buttons[i] = input.controllerstate[joy].Buttons[button]
	}
	return buttons
}

func (input *Input) GetJoystickPath(joy int) string {
	// Browsers do not expose a device path; reuse the id string.
	return input.GetJoystickName(joy)
}

var jsGUIDVendorRe = regexp.MustCompile(`(?i)vendor:\s*([0-9a-f]{4})`)
var jsGUIDProductRe = regexp.MustCompile(`(?i)product:\s*([0-9a-f]{4})`)

func (input *Input) GetJoystickGUID(joy int) string {
	pad := getGamepad(joy)
	if !pad.Truthy() {
		return ""
	}
	id := pad.Get("id").String()

	parseHex16 := func(re *regexp.Regexp) uint16 {
		if m := re.FindStringSubmatch(id); m != nil {
			var v uint16
			for _, c := range strings.ToLower(m[1]) {
				v <<= 4
				switch {
				case c >= '0' && c <= '9':
					v |= uint16(c - '0')
				case c >= 'a' && c <= 'f':
					v |= uint16(c-'a') + 10
				}
			}
			return v
		}
		return 0
	}

	vid := parseHex16(jsGUIDVendorRe)
	pid := parseHex16(jsGUIDProductRe)
	guid := make([]byte, 16)

	guid[0] = 0x03
	binary.LittleEndian.PutUint16(guid[4:6], vid)
	binary.LittleEndian.PutUint16(guid[8:10], pid)
	return hex.EncodeToString(guid[:])
}

func (input *Input) RumbleController(joy int, lo, hi uint16, ticks uint32) {
	if joy < 0 || joy >= len(input.controllers) || joy >= len(sys.joystickConfig) {
		return
	}
	if input.controllerstate[joy] == nil {
		return
	}

	// Only if Rumble Enabled for this config
	if input.controllerstate[joy].HasRumble && sys.joystickConfig[joy].rumbleOn {
		pad := getGamepad(joy)
		if !pad.Truthy() {
			return
		}
		actuator := pad.Get("vibrationActuator")
		if !actuator.Truthy() {
			return
		}

		gls := sys.gameLogicSpeed()
		if gls > 0 && sys.turbo > 0 {
			var framerate_ms uint32 = uint32(math.Ceil(1.0 / float64(gls) * float64(sys.turbo) * 1000.0))
			var buffertime_ms uint32 = framerate_ms >> 1 // makes rumble feel more consistent between frames

			if ticks > 0 {
				duration := (ticks * framerate_ms) + buffertime_ms
				actuator.Call("playEffect", "dual-rumble", map[string]interface{}{
					"duration":        int(duration),
					"strongMagnitude": float64(lo) / 65535.0,
					"weakMagnitude":   float64(hi) / 65535.0,
				})
			} else if actuator.Get("reset").Truthy() {
				actuator.Call("reset")
			}
		}
	}
}

func CheckAxisForDpad(axes *[6]float32, base int) string {
	var s string = ""

	// Left stick
	if (*axes)[0] > sys.cfg.Input.ControllerStickSensitivity { // right (LS)
		s = ButtonToStringLUT[2+base]
	} else if -(*axes)[0] > sys.cfg.Input.ControllerStickSensitivity { // left (LS)
		s = ButtonToStringLUT[1+base]
	}
	if (*axes)[1] > sys.cfg.Input.ControllerStickSensitivity { // down (LS)
		s = ButtonToStringLUT[3+base]
	} else if -(*axes)[1] > sys.cfg.Input.ControllerStickSensitivity { // up (LS)
		s = ButtonToStringLUT[base]
	}

	// Right stick
	if (*axes)[2] > sys.cfg.Input.ControllerStickSensitivity { // right (RS)
		s = ButtonToStringLUT[8+base]
	} else if -(*axes)[2] > sys.cfg.Input.ControllerStickSensitivity { // left (RS)
		s = ButtonToStringLUT[7+base]
	}
	if (*axes)[3] > sys.cfg.Input.ControllerStickSensitivity { // down (RS)
		s = ButtonToStringLUT[9+base]
	} else if -(*axes)[3] > sys.cfg.Input.ControllerStickSensitivity { // up (RS)
		s = ButtonToStringLUT[6+base]
	}

	return s
}

func CheckAxisForTrigger(axes *[6]float32) string {
	var s string = ""
	axesList := [2]int{4, 5} // left trigger, right trigger
	for _, i := range axesList {
		// No need for "stuck axis" behavior anymore
		if (*axes)[i] > 0 {
			s = ButtonToStringLUT[15+i]
			break
		}
	}
	return s
}

// returns the first active button/axis token string and the joystick index.
func getJoystickKey(controllerIdx int) (string, int) {
	var s string
	min, max := 0, input.GetMaxJoystickCount()

	if controllerIdx >= 0 && controllerIdx < max {
		min, max = controllerIdx, controllerIdx+1
	}

	for joy := min; joy < max; joy++ {
		if !input.IsJoystickPresent(joy) {
			continue
		}

		axes := input.GetJoystickAxes(joy)
		btns := input.GetJoystickButtons(joy)

		// Prefer stick directions (LS_*/RS_* / DP_*).
		s = CheckAxisForDpad(&axes, len(btns))
		if s != "" {
			return s, joy
		}

		// Then triggers (LT / RT).
		s = CheckAxisForTrigger(&axes)
		if s != "" {
			return s, joy
		}

		// Finally, digital buttons (A/B/X/Y/etc.).
		for i := range btns {
			if btns[i] > 0 {
				s = ButtonToStringLUT[i]
			}
		}
		if s != "" && strings.ToLower(s) != "not used" {
			return s, joy
		}
	}

	return "", -1
}
