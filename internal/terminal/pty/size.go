package pty

func normalizedSize(cols, rows uint16) (uint16, uint16) {
	if cols < 20 {
		cols = 20
	}
	if cols > 2000 {
		cols = 2000
	}
	if rows < 5 {
		rows = 5
	}
	if rows > 1000 {
		rows = 1000
	}
	return cols, rows
}
