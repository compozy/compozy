package session

func WaitingOnFor(signals []WorkSignal) WaitingOn {
	var reason WaitingOn
	for _, signal := range signals {
		switch signal.Kind {
		case WorkSignalActiveChild, WorkSignalAwaitingReply, WorkSignalLoopRun, WorkSignalTaskLease:
			return WaitingOnSessions
		case WorkSignalActiveMonitor, WorkSignalScheduledWait:
			reason = WaitingOnMonitor
		}
	}
	return reason
}

func waitingFromInfo(info *Info) bool {
	return info != nil && info.Supervision != nil && WaitingOnFor(info.Supervision.WorkSignals) != ""
}
