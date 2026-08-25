package workspacedb

// TerminalCommandWrite is the storage boundary for one immutable command row.
type TerminalCommandWrite struct {
	ID, ProfileID, ActorKind, ActorID string
	Command, Cwd, ExitCause           string
	DetectedBy, Approval              string
	TerminalID, SessionID, RunID      *string
	ArgvDigest, ExitSignal            *string
	RecordingID                       *string
	StartedAt, OutputBytes            int64
	DurationMs                        *int64
	ExitCode                          *int
	Truncated                         bool
}

// TerminalArtifactWrite is the storage boundary for retained command output.
type TerminalArtifactWrite struct {
	ID, CommandID, ProfileID, Digest, Path string
	TerminalID                             *string
	Bytes, ExpiresAt                       int64
}

// TerminalArtifactRecord is stored metadata for one retained command artifact.
type TerminalArtifactRecord struct {
	ID, CommandID, ProfileID, Digest, Path string
	TerminalID                             *string
	Bytes, ExpiresAt                       int64
}

// TerminalRecordingWrite is the storage boundary for one retained recording.
type TerminalRecordingWrite struct {
	ID, TerminalID, ProfileID, Digest, Path string
	StartedAt, Bytes, ExpiresAt             int64
	StoppedAt                               *int64
}

// TerminalRecordingRecord is stored metadata for one retained recording.
type TerminalRecordingRecord struct {
	ID, TerminalID, ProfileID, Digest, Path string
	StartedAt, Bytes, ExpiresAt             int64
	StoppedAt                               *int64
}
