package contract

// SessionFileKind distinguishes files from folders in session file search results.
type SessionFileKind string

const (
	SessionFileKindFile   SessionFileKind = "file"
	SessionFileKindFolder SessionFileKind = "folder"
)

// SessionFileKindValues lists the public session file kinds.
func SessionFileKindValues() []string {
	return []string{string(SessionFileKindFile), string(SessionFileKindFolder)}
}

// SessionFileEntryPayload is one ranked file or folder under a session's working directory.
type SessionFileEntryPayload struct {
	// Path is relative to the session's working directory; folder paths end in "/".
	Path string          `json:"path"`
	Name string          `json:"name"`
	Kind SessionFileKind `json:"kind"`
}

// SessionFilesResponse lists ranked files and folders for the composer mention picker.
type SessionFilesResponse struct {
	Entries []SessionFileEntryPayload `json:"entries"`
	// Truncated reports that the working directory exceeded the index cap.
	Truncated bool `json:"truncated"`
}
