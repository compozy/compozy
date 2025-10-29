package standalone

// GenerateLargeString returns a deterministic string of the requested size.
func GenerateLargeString(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = 'X'
	}
	return string(b)
}
