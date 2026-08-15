package attachments

import "encoding/binary"

func webpDimensions(data []byte) (int, int) {
	if !isWebP(data) {
		return 0, 0
	}
	offset := 12
	for offset+8 <= len(data) {
		chunk := string(data[offset : offset+4])
		size := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		payload := offset + 8
		end := payload + int(size)
		if end > len(data) {
			return 0, 0
		}
		switch chunk {
		case "VP8X":
			return vp8xDimensions(data[payload:end])
		case "VP8 ":
			return vp8Dimensions(data[payload:end])
		case "VP8L":
			return vp8lDimensions(data[payload:end])
		}
		offset = end + int(size&1)
	}
	return 0, 0
}

func vp8xDimensions(payload []byte) (int, int) {
	if len(payload) < 10 {
		return 0, 0
	}
	width := int(uint32(payload[4])|uint32(payload[5])<<8|uint32(payload[6])<<16) + 1
	height := int(uint32(payload[7])|uint32(payload[8])<<8|uint32(payload[9])<<16) + 1
	return width, height
}

func vp8Dimensions(payload []byte) (int, int) {
	if len(payload) < 10 {
		return 0, 0
	}
	if payload[3] != 0x9d || payload[4] != 0x01 || payload[5] != 0x2a {
		return 0, 0
	}
	width := int(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
	height := int(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
	return width, height
}

func vp8lDimensions(payload []byte) (int, int) {
	if len(payload) < 5 || payload[0] != 0x2f {
		return 0, 0
	}
	bits := binary.LittleEndian.Uint32(payload[1:5])
	width := int(bits&0x3fff) + 1
	height := int((bits>>14)&0x3fff) + 1
	return width, height
}
