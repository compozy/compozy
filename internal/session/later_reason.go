package session

import (
	"bytes"
	"regexp"
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

// laterReasonLineBreakTag matches an inline `<br>`, `<br/>`, or `<br />`.
var laterReasonLineBreakTag = regexp.MustCompile(`(?i)^<br\s*/?>$`)

// laterReasonMarkdown parses reasons as CommonMark plus GFM strikethrough and
// tables, matching the web reader.
var laterReasonMarkdown = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Table))

// LaterReasonPlainText reads a Markdown Later reason as one plain line for
// surfaces that cannot render Markdown: the reason is parsed as CommonMark,
// its text (link labels, code spans, escaped characters, code block lines)
// stays literal, its markup (including raw HTML) drops, and each paragraph, list item, or code line
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

// visit routes block structure here and inline content to visitInline.
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
	case *ast.HTMLBlock:
		if entering {
			r.flush()
		}
		return ast.WalkSkipChildren, nil
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		if entering {
			r.writeLines(node)
		}
		return ast.WalkSkipChildren, nil
	default:
		return r.visitInline(node, entering), nil
	}
	return ast.WalkContinue, nil
}

// visitInline reads inline text literally and skips inline markup.
func (r *laterReasonReader) visitInline(node ast.Node, entering bool) ast.WalkStatus {
	switch typed := node.(type) {
	case *ast.Text:
		if entering {
			r.current.Write(laterReasonProse(typed.Segment.Value(r.source)))
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
		return ast.WalkSkipChildren
	case *ast.AutoLink:
		if entering {
			r.current.Write(typed.Label(r.source))
		}
		return ast.WalkSkipChildren
	case *ast.RawHTML:
		// Inline tags are markup; the text between them is read as usual. A
		// line-break tag still separates words, so it ends the part.
		if entering && laterReasonLineBreakTag.Match(typed.Segments.Value(r.source)) {
			r.flush()
		}
		return ast.WalkSkipChildren
	}
	return ast.WalkContinue
}

// laterReasonProse reads a prose text segment as rendered: a backslash escape
// becomes its literal character, and character references elsewhere (`&amp;`,
// `&#38;`) decode. An escaped `\&amp;` therefore stays `&amp;`.
func laterReasonProse(value []byte) []byte {
	out := make([]byte, 0, len(value))
	start := 0
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) || !util.IsPunct(value[index+1]) {
			continue
		}
		out = append(out, resolveCharacterReferences(value[start:index])...)
		out = append(out, value[index+1])
		index++
		start = index + 1
	}
	return append(out, resolveCharacterReferences(value[start:])...)
}

// resolveCharacterReferences decodes complete character references in one
// pass, so decoded text is never read again: `&#38;copy;` stays `&copy;`.
func resolveCharacterReferences(chunk []byte) []byte {
	out := make([]byte, 0, len(chunk))
	for len(chunk) > 0 {
		amp := bytes.IndexByte(chunk, '&')
		if amp < 0 {
			return append(out, chunk...)
		}
		out = append(out, chunk[:amp]...)
		chunk = chunk[amp:]
		// A reference ends at the first semicolon, so this candidate holds at
		// most one reference.
		if semi := bytes.IndexByte(chunk, ';'); semi > 0 {
			candidate := chunk[:semi+1]
			decoded := util.ResolveNumericReferences(candidate)
			if bytes.Equal(decoded, candidate) {
				decoded = util.ResolveEntityNames(candidate)
			}
			if !bytes.Equal(decoded, candidate) {
				out = append(out, decoded...)
				chunk = chunk[semi+1:]
				continue
			}
		}
		out = append(out, '&')
		chunk = chunk[1:]
	}
	return out
}
