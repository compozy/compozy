package postgres

import "strings"

// dollarList builds a comma-separated $n list starting at start with n items.
// Example: dollarList(1,3) -> "$1,$2,$3"
func dollarList(start, n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "$" + itoa(start+i)
	}
	return strings.Join(parts, ",")
}

func itoa(i int) string {
	// Avoid importing strconv to keep this tiny helper inline.
	// i is small (placeholders count), so this is sufficient.
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
