package rules

import "regexp"

var channelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// ValidChannel reports whether the channel matches the shared network grammar.
func ValidChannel(channel string) bool {
	return channelPattern.MatchString(channel)
}
