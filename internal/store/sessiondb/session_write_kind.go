package sessiondb

type sessionWriteKind int

const (
	sessionWriteEvent sessionWriteKind = iota + 1
	sessionWriteEventIfAbsent
	sessionWriteEventBatch
	sessionWriteUsage
	sessionWriteHookRun
	sessionWriteArchive
	sessionWriteClear
)
