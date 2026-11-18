package contract

type FileFindMatch struct {
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	Sensitive   bool    `json:"sensitive,omitzero"`
	Score       int     `json:"score"`
	MatchRanges [][]int `json:"match_ranges"`
}
type FileFindResponse struct {
	Root           FileRootPayload `json:"root"`
	Query          string          `json:"query"`
	Results        []FileFindMatch `json:"results"`
	IndexedFiles   int             `json:"indexed_files"`
	IndexTruncated bool            `json:"index_truncated"`
}
type FileSearchQuery struct {
	Pattern       string   `json:"pattern"`
	Regex         bool     `json:"regex"`
	CaseSensitive bool     `json:"case_sensitive"`
	WholeWord     bool     `json:"whole_word"`
	Include       []string `json:"include"`
	Exclude       []string `json:"exclude"`
}
type FileSearchMatch struct {
	Line       int     `json:"line"`
	Column     int     `json:"column"`
	Text       string  `json:"text"`
	Ranges     [][]int `json:"ranges"`
	TextOffset int     `json:"text_offset"`
}
type FileSearchFile struct {
	Path             string            `json:"path"`
	Matches          []FileSearchMatch `json:"matches"`
	TruncatedMatches bool              `json:"truncated_matches,omitzero"`
}
type FileSearchTotals struct {
	Files   int `json:"files"`
	Matches int `json:"matches"`
}
type FileSearchResponse struct {
	Root              FileRootPayload  `json:"root"`
	Query             FileSearchQuery  `json:"query"`
	Engine            string           `json:"engine"`
	Files             []FileSearchFile `json:"files"`
	Totals            FileSearchTotals `json:"totals"`
	Truncated         bool             `json:"truncated"`
	TimedOut          bool             `json:"timed_out"`
	SensitiveExcluded int              `json:"sensitive_excluded"`
}
