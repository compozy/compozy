package terminal

import "bytes"

func modelFacingOutput(input []byte) []byte {
	output := make([]byte, 0, len(input))
	for offset := 0; offset < len(input); {
		start, kind, prefix := controlStart(input[offset:])
		if start < 0 {
			output = append(output, input[offset:]...)
			break
		}
		output = append(output, input[offset:offset+start]...)
		offset += start
		end, terminator := controlEnd(input[offset+prefix:], kind)
		if end < 0 {
			if kind == 'o' && !blockedModelOSC(input[offset+prefix:]) {
				output = append(output, input[offset:]...)
			}
			break
		}
		wholeEnd := offset + prefix + end + terminator
		if kind == 'o' && !blockedModelOSC(input[offset+prefix:wholeEnd]) {
			output = append(output, input[offset:wholeEnd]...)
		}
		offset = wholeEnd
	}
	return output
}

func blockedModelOSC(content []byte) bool {
	command, _, _ := bytes.Cut(content, []byte{';'})
	return bytes.Equal(command, []byte("7")) || bytes.Equal(command, []byte("8"))
}
