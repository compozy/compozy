package acp

type processChildTask struct {
	done chan struct{}
}

func (p *AgentProcess) beginChildTask() (*processChildTask, bool) {
	if p == nil {
		return nil, false
	}
	p.childTasksMu.Lock()
	defer p.childTasksMu.Unlock()
	if p.childTasksClosing {
		return nil, false
	}
	if p.childTasks == nil {
		p.childTasks = make(map[*processChildTask]struct{})
	}
	run := &processChildTask{done: make(chan struct{})}
	p.childTasks[run] = struct{}{}
	return run, true
}

func (p *AgentProcess) finishChildTask(run *processChildTask) {
	if p == nil || run == nil {
		return
	}
	p.childTasksMu.Lock()
	if _, ok := p.childTasks[run]; ok {
		delete(p.childTasks, run)
		close(run.done)
	}
	p.childTasksMu.Unlock()
}

func (p *AgentProcess) startReservedChildTask(run *processChildTask, task func()) {
	go func() {
		defer p.finishChildTask(run)
		task()
	}()
}

func (p *AgentProcess) closeAndWaitChildTasks() {
	if p == nil {
		return
	}
	p.childTasksMu.Lock()
	p.childTasksClosing = true
	done := make([]<-chan struct{}, 0, len(p.childTasks))
	for run := range p.childTasks {
		done = append(done, run.done)
	}
	p.childTasksMu.Unlock()
	for _, runDone := range done {
		<-runDone
	}
}
