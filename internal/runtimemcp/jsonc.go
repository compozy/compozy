package runtimemcp

import "bytes"

func stripJSONComments(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	out := bytes.Clone(data)
	if !removeJSONComments(out) {
		return data // Preserve invalid comments so the JSON decoder rejects the file.
	}
	removeTrailingCommas(out)
	return out
}

func jsonStringEnd(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return len(data)
}

func removeJSONComments(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			i = jsonStringEnd(data, i)
			continue
		}
		if data[i] != '/' || i+1 >= len(data) {
			continue
		}
		end := i
		switch data[i+1] {
		case '/':
			end = bytes.IndexByte(data[i:], '\n')
			if end < 0 {
				end = len(data) - i
			}
			end += i
		case '*':
			end = bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return false
			}
			end += i + 4
		default:
			continue
		}
		for j := i; j < end; j++ {
			if data[j] != '\n' && data[j] != '\r' {
				data[j] = ' '
			}
		}
		i = end - 1
	}
	return true
}

func removeTrailingCommas(data []byte) {
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			i = jsonStringEnd(data, i)
			continue
		}
		if data[i] != ',' {
			continue
		}
		rest := bytes.TrimSpace(data[i+1:])
		if len(rest) > 0 && (rest[0] == '}' || rest[0] == ']') {
			data[i] = ' '
		}
	}
}
