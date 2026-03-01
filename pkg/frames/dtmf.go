// Package frames defines DTMF and IVR-related frame types.
package frames

import (
	"fmt"
	"strings"
)

// KeypadEntry represents a valid DTMF key (0-9, *, #).
type KeypadEntry string

// Valid DTMF characters.
const (
	KeypadStar  KeypadEntry = "*"
	KeypadPound KeypadEntry = "#"
)

// ValidKeypadRunes is the set of valid single-keypad characters.
var ValidKeypadRunes = "0123456789*#"

// ParseKeypadEntry parses a string into a KeypadEntry. The input may be
// a single character or multiple digits (e.g. "1", "123"). Each rune must
// be 0-9, *, or #. For multi-digit sequences, only the first character
// is used for the KeypadEntry; use ParseKeypadSequence for full sequences.
func ParseKeypadEntry(s string) (KeypadEntry, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty keypad entry")
	}
	for _, r := range s {
		if !strings.ContainsRune(ValidKeypadRunes, r) {
			return "", fmt.Errorf("invalid keypad character: %q (must be 0-9, *, or #)", r)
		}
	}
	return KeypadEntry(string(s[0])), nil
}

// String returns the keypad character as string.
func (k KeypadEntry) String() string { return string(k) }

// InputDTMFFrame carries an inbound DTMF keypress from the transport (e.g. telephony).
// Pipeline/processors can use this for IVR or user input.
type InputDTMFFrame struct {
	ControlFrame
	Digit KeypadEntry `json:"digit"`
}

func (*InputDTMFFrame) FrameType() string { return "InputDTMFFrame" }

// NewInputDTMFFrame creates an InputDTMFFrame. Returns error if digit is not valid (0-9, *, #).
func NewInputDTMFFrame(digit KeypadEntry) (*InputDTMFFrame, error) {
	if digit == "" {
		return nil, fmt.Errorf("digit cannot be empty")
	}
	if !strings.ContainsRune(ValidKeypadRunes, rune(digit[0])) {
		return nil, fmt.Errorf("invalid DTMF digit: %q", digit)
	}
	return &InputDTMFFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		Digit:        digit,
	}, nil
}

// OutputDTMFUrgentFrame carries a DTMF keypress for the transport to play
// (e.g. for IVR navigation). Transport implementations may emit the actual
// DTMF tone or forward it to the telephony layer.
type OutputDTMFUrgentFrame struct {
	ControlFrame
	Button KeypadEntry `json:"button"`
}

func (*OutputDTMFUrgentFrame) FrameType() string { return "OutputDTMFUrgentFrame" }

// NewOutputDTMFUrgentFrame creates an OutputDTMFUrgentFrame. Returns error if
// button is not a valid keypad entry (0-9, *, #).
func NewOutputDTMFUrgentFrame(button KeypadEntry) (*OutputDTMFUrgentFrame, error) {
	if button == "" {
		return nil, fmt.Errorf("button cannot be empty")
	}
	if !strings.ContainsRune(ValidKeypadRunes, rune(button[0])) {
		return nil, fmt.Errorf("invalid DTMF button: %q", button)
	}
	return &OutputDTMFUrgentFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		Button:       button,
	}, nil
}
