package control

import "strings"

var reservedHostedParts = map[string]struct{}{
	"account": {}, "admin": {}, "amazon": {}, "apple": {}, "auth": {}, "bank": {},
	"billing": {}, "cloudflare": {}, "compozy": {}, "facebook": {}, "github": {},
	"google": {}, "icloud": {}, "login": {}, "microsoft": {}, "netflix": {},
	"office": {}, "paypal": {}, "signin": {}, "stripe": {}, "support": {},
	"verify": {}, "visa": {}, "www": {},
}

func hostedHostname(name, suffix, domain string) string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, name)
	var slug strings.Builder
	for part := range strings.SplitSeq(normalized, "-") {
		if part == "" {
			continue
		}
		if _, reserved := reservedHostedParts[part]; reserved {
			return "computer-" + suffix + "." + domain
		}
		if slug.Len() > 0 {
			slug.WriteByte('-')
		}
		slug.WriteString(part)
	}
	label := "computer"
	if slug.Len() > 0 {
		// Keep room for the separator and four ID characters in a DNS label.
		label = strings.TrimRight(slug.String()[:min(slug.Len(), 58)], "-")
	}
	for part := range strings.SplitSeq(label, "-") {
		if _, reserved := reservedHostedParts[part]; reserved {
			label = "computer"
			break
		}
	}
	return label + "-" + suffix + "." + domain
}
