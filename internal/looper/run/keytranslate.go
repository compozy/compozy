package run

import tea "github.com/charmbracelet/bubbletea"

var (
	keyEnter      = []byte{0x0d}
	keyTab        = []byte{0x09}
	keyBackspace  = []byte{0x7f}
	keyArrowUp    = []byte{0x1b, '[', 'A'}
	keyArrowDown  = []byte{0x1b, '[', 'B'}
	keyArrowRight = []byte{0x1b, '[', 'C'}
	keyArrowLeft  = []byte{0x1b, '[', 'D'}
	keyCtrlC      = []byte{0x03}
	keyCtrlD      = []byte{0x04}
	keyEsc        = []byte{0x1b}
)

// translateKey converts Bubble Tea key events into the bytes expected by a PTY.
func translateKey(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyEnter:
		return append([]byte(nil), keyEnter...)
	case tea.KeyTab:
		return append([]byte(nil), keyTab...)
	case tea.KeyBackspace:
		return append([]byte(nil), keyBackspace...)
	case tea.KeyUp:
		return append([]byte(nil), keyArrowUp...)
	case tea.KeyDown:
		return append([]byte(nil), keyArrowDown...)
	case tea.KeyRight:
		return append([]byte(nil), keyArrowRight...)
	case tea.KeyLeft:
		return append([]byte(nil), keyArrowLeft...)
	case tea.KeyCtrlC:
		return append([]byte(nil), keyCtrlC...)
	case tea.KeyCtrlD:
		return append([]byte(nil), keyCtrlD...)
	case tea.KeyEsc:
		return append([]byte(nil), keyEsc...)
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return nil
		}
		return []byte(string(msg.Runes))
	default:
		return nil
	}
}
