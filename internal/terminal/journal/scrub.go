package journal

import (
	"regexp"

	"github.com/compozy/compozy/internal/redact"
)

var mysqlInlinePassword = regexp.MustCompile(`(?i)(^|\s)(-p)([^\s]+)`)

func scrubCommand(command string) string {
	scrubbed := redact.String(command)
	return mysqlInlinePassword.ReplaceAllString(scrubbed, "${1}${2}"+redact.Marker)
}
