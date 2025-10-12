package chunk

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

func stripHTML(input string) string {
	if !strings.ContainsAny(input, "<>") {
		return input
	}
	node, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	output := builder.String()
	return strings.TrimFunc(output, unicode.IsSpace)
}
