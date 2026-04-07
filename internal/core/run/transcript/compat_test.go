package transcript

func newSessionViewModel() *ViewModel {
	return NewViewModel()
}

const (
	transcriptEntryAssistantMessage = EntryKindAssistantMessage
	transcriptEntryToolCall         = EntryKindToolCall
)
