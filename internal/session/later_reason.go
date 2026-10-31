package session

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// laterReasonLineSeparator joins the lines of a multi-line reason when it is
// read as one line (a notification body).
const laterReasonLineSeparator = " · "

// laterReasonMarkdown parses reasons as CommonMark plus GFM strikethrough and
// tables, matching the web reader.
var laterReasonMarkdown = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Table))

// LaterReasonPlainText reads a Markdown Later reason as one plain line for
// surfaces that cannot render Markdown: the reason is parsed as CommonMark,
// its text (link labels, code spans, escaped characters, code block lines)
// stays literal, its markup drops, and each paragraph, list item, or code line
// becomes one part joined with a middle dot. The stored reason is never
// rewritten.
func LaterReasonPlainText(reason string) string {
	source := []byte(strings.ReplaceAll(reason, "\r\n", "\n"))
	reader := laterReasonReader{source: source}
	document := laterReasonMarkdown.Parser().Parse(text.NewReader(source))
	if err := ast.Walk(document, reader.visit); err != nil {
		return strings.Join(strings.Fields(reason), " ")
	}
	reader.flush()
	return strings.Join(reader.lines, laterReasonLineSeparator)
}

type laterReasonReader struct {
	source  []byte
	current strings.Builder
	lines   []string
}

func (r *laterReasonReader) flush() {
	if line := strings.Join(strings.Fields(r.current.String()), " "); line != "" {
		r.lines = append(r.lines, line)
	}
	r.current.Reset()
}

func (r *laterReasonReader) writeLines(node ast.Node) {
	r.flush()
	segments := node.Lines()
	for index := range segments.Len() {
		segment := segments.At(index)
		r.current.Write(segment.Value(r.source))
		r.flush()
	}
}

func (r *laterReasonReader) visit(node ast.Node, entering bool) (ast.WalkStatus, error) {
	switch typed := node.(type) {
	case *ast.Paragraph, *ast.TextBlock, *ast.Heading, *extensionast.TableHeader, *extensionast.TableRow:
		if !entering {
			r.flush()
		}
	case *extensionast.TableCell:
		// A table row reads as its cells joined with the line separator.
		if entering && typed.PreviousSibling() != nil {
			r.current.WriteString(laterReasonLineSeparator)
		}
	case *ast.Text:
		if entering {
			// Prose keeps escaped characters without their backslash, as rendered.
			r.current.Write(util.UnescapePunctuations(typed.Segment.Value(r.source)))
			if typed.SoftLineBreak() || typed.HardLineBreak() {
				r.flush()
			}
		}
	case *ast.String:
		if entering {
			r.current.Write(typed.Value)
		}
	case *ast.CodeSpan:
		if entering {
			for child := typed.FirstChild(); child != nil; child = child.NextSibling() {
				if segment, ok := child.(*ast.Text); ok {
					r.current.Write(segment.Segment.Value(r.source))
				}
			}
		}
		return ast.WalkSkipChildren, nil
	case *ast.AutoLink:
		if entering {
			r.current.Write(typed.Label(r.source))
		}
		return ast.WalkSkipChildren, nil
	case *ast.RawHTML:
		if entering {
			for index := range typed.Segments.Len() {
				segment := typed.Segments.At(index)
				r.current.Write(segment.Value(r.source))
			}
		}
		return ast.WalkSkipChildren, nil
	case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.HTMLBlock:
		if entering {
			r.writeLines(node)
		}
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}
