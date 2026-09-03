package terminal

import "bytes"

func modelFacingOutput(input []byte) []byte {
	output := make([]byte, 0, len(input))
	for offset := 0; offset < len(input); {
		kind, prefix, ok := modelControlAt(input[offset:])
		if !ok {
			output = append(output, input[offset])
			offset++
			continue
		}
		end := modelControlEnd(input[offset+prefix:], kind == modelControlOSC)
		if end < 0 {
			if kind == modelControlOSC && !blockedModelOSC(input[offset+prefix:]) {
				output = append(output, input[offset:]...)
			}
			break
		}
		wholeEnd := offset + prefix + end
		if kind == modelControlOSC && !blockedModelOSC(input[offset+prefix:wholeEnd]) {
			output = append(output, input[offset:wholeEnd]...)
		}
		offset = wholeEnd
	}
	return output
}

type modelControl uint8

const (
	modelControlOSC modelControl = iota + 1
	modelControlDCS
)

func modelControlAt(input []byte) (modelControl, int, bool) {
	if len(input) >= 2 && input[0] == 0x1b {
		switch input[1] {
		case ']':
			return modelControlOSC, 2, true
		case 'P':
			return modelControlDCS, 2, true
		}
	}
	if len(input) > 0 {
		switch input[0] {
		case 0x9d:
			return modelControlOSC, 1, true
		case 0x90:
			return modelControlDCS, 1, true
		}
	}
	return 0, 0, false
}

func modelControlEnd(input []byte, allowBell bool) int {
	for index, value := range input {
		if allowBell && value == 0x07 {
			return index + 1
		}
		if value == 0x9c {
			return index + 1
		}
		if value == 0x1b && index+1 < len(input) && input[index+1] == '\\' {
			return index + 2
		}
	}
	return -1
}

func blockedModelOSC(content []byte) bool {
	command, _, _ := bytes.Cut(content, []byte{';'})
	return bytes.Equal(command, []byte("7")) || bytes.Equal(command, []byte("8"))
}
