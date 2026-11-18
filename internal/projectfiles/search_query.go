package projectfiles

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar"
)

type searchPattern struct {
	query SearchQuery
	regex *regexp.Regexp
}

func compileSearch(q SearchQuery) (searchPattern, error) {
	p := searchPattern{query: q}
	if !utf8.ValidString(q.Pattern) || utf8.RuneCountInString(q.Pattern) < 2 ||
		utf8.RuneCountInString(q.Pattern) > 512 ||
		strings.ContainsAny(q.Pattern, "\x00\n\r") {
		return p, ErrInvalidQuery
	}
	for _, globs := range [][]string{q.Include, q.Exclude} {
		if len(globs) > 10 {
			return p, ErrInvalidQuery
		}
		for _, glob := range globs {
			if glob == "" || strings.HasPrefix(glob, ":") || strings.HasPrefix(glob, "/") ||
				strings.ContainsAny(glob, "\\\x00\n\r") {
				return p, ErrInvalidQuery
			}
			for part := range strings.SplitSeq(glob, "/") {
				if part == ".." {
					return p, ErrInvalidQuery
				}
			}
			if _, err := path.Match(glob, ""); err != nil {
				return p, ErrInvalidQuery
			}
		}
	}
	if q.Regex {
		compiled, err := regexp.CompilePOSIX(q.Pattern)
		if err != nil {
			return p, fmt.Errorf("%w: %s", ErrInvalidQuery, err)
		}
		p.regex = compiled
	}
	if !q.CaseSensitive {
		expression := q.Pattern
		if !q.Regex {
			expression = regexp.QuoteMeta(expression)
		}
		compiled, err := regexp.Compile("(?i:" + expression + ")")
		if err != nil {
			return p, fmt.Errorf("%w: %s", ErrInvalidQuery, err)
		}
		compiled.Longest()
		p.regex = compiled
	}

	return p, nil
}
func (p searchPattern) accepts(path string) bool {
	included := len(p.query.Include) == 0
	for _, glob := range p.query.Include {
		match, err := doublestar.Match(glob, path)
		if err != nil {
			return false
		}
		included = included || match
	}
	for _, glob := range p.query.Exclude {
		match, err := doublestar.Match(glob, path)
		if err != nil {
			return false
		}
		if match {
			return false
		}
	}
	return included
}
func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
func wholeWord(line string, start, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(line[:start])
	after, _ := utf8.DecodeRuneInString(line[end:])
	return (start == 0 || !wordRune(before)) && (end == len(line) || !wordRune(after))
}
func (p searchPattern) ranges(line string) [][2]int {
	text, pattern := line, p.query.Pattern
	// Case folding preserves original byte offsets by compiling case-insensitive Go regexes.
	var found [][]int
	if p.regex != nil {
		found = p.regex.FindAllStringIndex(text, -1)
	} else {
		for offset := 0; offset <= len(text); {
			i := strings.Index(text[offset:], pattern)
			if i < 0 {
				break
			}
			start := offset + i
			found = append(found, []int{start, start + len(pattern)})
			offset = start + len(pattern)
		}
	}

	ranges := make([][2]int, 0, len(found))
	for _, r := range found {
		if p.query.WholeWord && !wholeWord(line, r[0], r[1]) {
			continue
		}
		ranges = append(ranges, [2]int{r[0], r[1]})
	}
	return ranges
}
func excerpt(line string, ranges [][2]int) (string, [][2]int, int) {
	offsets := make([]int, 0, 301)
	for i := range line {
		offsets = append(offsets, i)
	}
	offsets = append(offsets, len(line))
	if len(offsets) <= 301 {
		return line, ranges, 0
	}
	first := 0
	for first+1 < len(offsets) && offsets[first] < ranges[0][0] {
		first++
	}
	start := max(0, first-100)
	end := min(len(offsets)-1, start+300)
	start = max(0, end-300)
	from, to := offsets[start], offsets[end]
	clipped := make([][2]int, 0, len(ranges))
	for _, r := range ranges {
		if r[0] >= from && r[0] < to {
			clipped = append(clipped, [2]int{r[0] - from, min(r[1], to) - from})
		}
	}
	return line[from:to], clipped, from
}
