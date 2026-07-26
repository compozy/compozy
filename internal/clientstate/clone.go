package clientstate

func cloneEntry(entry Entry) Entry {
	entry.Value = append([]byte(nil), entry.Value...)
	return entry
}

func cloneEntries(entries []Entry) []Entry {
	cloned := make([]Entry, len(entries))
	for index, entry := range entries {
		cloned[index] = cloneEntry(entry)
	}
	return cloned
}

func cloneEvent(event Event) Event {
	event.Entry = cloneEntry(event.Entry)
	return event
}
