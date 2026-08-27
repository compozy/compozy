package terminal

import (
	"bytes"
	"slices"
	"strconv"
)

const modeSequenceCarryLimit = 128

var replayableDECModeOrder = []int{1, 7, 25, 47, 1047, 1049, 1004, 2004}

type modePreambleTracker struct {
	carry       []byte
	decModes    map[int]bool
	keypad      bool
	keypadKnown bool
}

func newModePreambleTracker() *modePreambleTracker {
	return &modePreambleTracker{decModes: make(map[int]bool)}
}

func (t *modePreambleTracker) Observe(input []byte) []byte {
	if t == nil || len(input) == 0 {
		return t.preamble()
	}
	combined := make([]byte, 0, len(t.carry)+len(input))
	combined = append(combined, t.carry...)
	combined = append(combined, input...)
	for index := 0; index < len(combined); index++ {
		if combined[index] != 0x1b {
			continue
		}
		if index+1 >= len(combined) {
			break
		}
		switch combined[index+1] {
		case 'c':
			clear(t.decModes)
			t.keypadKnown = false
			index++
		case '=':
			t.keypad, t.keypadKnown = true, true
			index++
		case '>':
			t.keypad, t.keypadKnown = false, true
			index++
		case '[':
			index = t.observeCSI(combined, index+2)
		}
	}
	carryStart := max(0, len(combined)-modeSequenceCarryLimit)
	t.carry = append(t.carry[:0], combined[carryStart:]...)
	return t.preamble()
}

func (t *modePreambleTracker) observeCSI(input []byte, start int) int {
	if start >= len(input) || input[start] != '?' {
		return start - 1
	}
	paramsStart := start + 1
	for index := paramsStart; index < len(input); index++ {
		value := input[index]
		if value >= '0' && value <= '9' || value == ';' {
			continue
		}
		if value != 'h' && value != 'l' {
			return index
		}
		on := value == 'h'
		for raw := range bytes.SplitSeq(input[paramsStart:index], []byte{';'}) {
			mode, err := strconv.Atoi(string(raw))
			if err == nil && replayableDECMode(mode) {
				t.decModes[mode] = on
			}
		}
		return index
	}
	return len(input) - 1
}

func replayableDECMode(mode int) bool {
	return slices.Contains(replayableDECModeOrder, mode)
}

func (t *modePreambleTracker) preamble() []byte {
	if t == nil {
		return nil
	}
	result := make([]byte, 0, 96)
	for _, mode := range replayableDECModeOrder {
		on, known := t.decModes[mode]
		if !known {
			continue
		}
		result = append(result, "\x1b[?"...)
		result = strconv.AppendInt(result, int64(mode), 10)
		if on {
			result = append(result, 'h')
		} else {
			result = append(result, 'l')
		}
	}
	if t.keypadKnown {
		result = append(result, 0x1b)
		if t.keypad {
			result = append(result, '=')
		} else {
			result = append(result, '>')
		}
	}
	return result
}
