package terminal

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPendingOSCBytes = 64 << 10

type outputFilter interface {
	Filter(input []byte) FilterResult
	Flush() FilterResult
	FilterInput(input []byte) []byte
}

type oscSecurityFilter struct {
	nonce   string
	onTitle func(string)
	output  oscParser
	input   oscParser
}

type oscParser struct {
	pending       []byte
	utf8Carry     []byte
	discarding    bool
	discardEscape bool
	discardKind   byte
}

func newOSCSecurityFilter(nonce string, onTitle func(string)) *oscSecurityFilter {
	return &oscSecurityFilter{nonce: nonce, onTitle: onTitle}
}

func (f *oscSecurityFilter) Filter(input []byte) FilterResult {
	display, facts := f.output.filter(input, f.nonce, f.onTitle, true, false)
	return FilterResult{DisplayBytes: display, MarkerFacts: facts}
}

func (f *oscSecurityFilter) Flush() FilterResult {
	display, facts := f.output.filter(nil, f.nonce, f.onTitle, true, true)
	return FilterResult{DisplayBytes: display, MarkerFacts: facts}
}

func (f *oscSecurityFilter) FilterInput(input []byte) []byte {
	display, _ := f.input.filter(input, "", nil, false, false)
	return display
}

func (p *oscParser) filter(
	input []byte,
	nonce string,
	onTitle func(string),
	parseMarkers bool,
	final bool,
) ([]byte, []MarkerFacts) {
	data := input
	if len(p.utf8Carry) > 0 {
		p.utf8Carry = append(p.utf8Carry, input...)
		data = p.utf8Carry
		p.utf8Carry = nil
	}
	if !final {
		var carry []byte
		data, carry = splitCompleteUTF8(data)
		p.utf8Carry = bytes.Clone(carry)
	}
	if p.discarding {
		data = p.discardUntilTerminator(data)
		if p.discarding {
			return nil, nil
		}
	}
	data = append(p.pending, data...)
	p.pending = nil
	display := make([]byte, 0, len(data))
	facts := make([]MarkerFacts, 0, 1)
	for len(data) > 0 {
		start, kind, prefixBytes := controlStart(data)
		if start < 0 {
			if len(data) > 0 && data[len(data)-1] == 0x1b {
				display = append(display, data[:len(data)-1]...)
				p.pending = append(p.pending, 0x1b)
				break
			}
			display = append(display, data...)
			break
		}
		display = append(display, data[:start]...)
		end, terminatorBytes := controlEnd(data[start+prefixBytes:], kind)
		if end < 0 {
			partial := data[start:]
			if len(partial) > maxPendingOSCBytes {
				p.discarding = true
				p.discardEscape = partial[len(partial)-1] == 0x1b
				p.discardKind = kind
			} else {
				p.pending = append(p.pending, partial...)
			}
			break
		}
		content := data[start+prefixBytes : start+prefixBytes+end]
		wholeEnd := start + prefixBytes + end + terminatorBytes
		if kind == 'd' {
			data = data[wholeEnd:]
			continue
		}
		switch {
		case bytes.HasPrefix(content, []byte("52;")):
		case bytes.HasPrefix(content, []byte("7;")), bytes.HasPrefix(content, []byte("8;")):
		case bytes.HasPrefix(content, []byte("0;")), bytes.HasPrefix(content, []byte("2;")):
			if onTitle != nil {
				onTitle(SanitizeTitle(string(content[2:])))
			}
		case bytes.HasPrefix(content, []byte("7113;")):
			if parseMarkers {
				if fact, ok := parseMarkerFact(string(content), nonce); ok {
					facts = append(facts, fact)
				}
			}
		default:
			display = append(display, data[start:wholeEnd]...)
		}
		data = data[wholeEnd:]
	}
	return display, facts
}

func (p *oscParser) discardUntilTerminator(input []byte) []byte {
	if p.discardEscape && len(input) > 0 && input[0] == '\\' {
		p.discarding = false
		p.discardEscape = false
		return input[1:]
	}
	p.discardEscape = false
	end, terminatorBytes := controlEnd(input, p.discardKind)
	if end < 0 {
		p.discardEscape = len(input) > 0 && input[len(input)-1] == 0x1b
		return nil
	}
	p.discarding = false
	return input[end+terminatorBytes:]
}

func controlStart(input []byte) (int, byte, int) {
	for index := 0; index < len(input); {
		value, size := utf8.DecodeRune(input[index:])
		// Decode first so continuation bytes cannot masquerade as controls.
		if value == utf8.RuneError && size == 1 {
			value = rune(input[index])
		}
		switch value {
		case 0x9d:
			return index, 'o', size
		case 0x90:
			return index, 'd', size
		}
		if value == 0x1b && index+1 < len(input) {
			switch input[index+1] {
			case ']':
				return index, 'o', 2
			case 'P':
				return index, 'd', 2
			}
		}
		index += size
	}
	return -1, 0, 0
}

func controlEnd(input []byte, kind byte) (int, int) {
	for index := 0; index < len(input); {
		value, size := utf8.DecodeRune(input[index:])
		if value == utf8.RuneError && size == 1 {
			value = rune(input[index])
		}
		if kind == 'o' && value == 0x07 {
			return index, 1
		}
		if value == 0x9c {
			return index, size
		}
		if value == 0x1b && index+1 < len(input) && input[index+1] == '\\' {
			return index, 2
		}
		index += size
	}
	return -1, 0
}

func parseMarkerFact(content, nonce string) (MarkerFacts, bool) {
	parts := strings.Split(content, ";")
	if len(parts) < 5 || parts[0] != "7113" || parts[1] != "v1" || parts[2] == "" || parts[2] != nonce {
		return MarkerFacts{}, false
	}
	fact := MarkerFacts{Kind: parts[3]}
	switch fact.Kind {
	case "S":
		values := markerValues(parts[4:])
		command, commandErr := url.PathUnescape(values["cmd"])
		cwd, cwdErr := url.PathUnescape(values["cwd"])
		if commandErr != nil || cwdErr != nil || command == "" || cwd == "" {
			return MarkerFacts{}, false
		}
		fact.Command, fact.Cwd = command, cwd
	case "F":
		values := markerValues(parts[4:])
		exitCode, err := strconv.Atoi(values["exit"])
		if err != nil {
			return MarkerFacts{}, false
		}
		fact.Exit = &exitCode
	default:
		return MarkerFacts{}, false
	}
	return fact, true
}

func markerValues(parts []string) map[string]string {
	values := make(map[string]string, len(parts))
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func SanitizeTitle(input string) string {
	var builder strings.Builder
	for _, value := range strings.ToValidUTF8(input, "") {
		if unicode.Is(unicode.C, value) {
			continue
		}
		if builder.Len()+utf8.RuneLen(value) > 256 {
			break
		}
		builder.WriteRune(value)
	}
	return strings.TrimSpace(builder.String())
}
