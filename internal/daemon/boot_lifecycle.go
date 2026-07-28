package daemon

func (d *Daemon) finishBoot(err *error) {
	if err == nil || *err == nil {
		return
	}
	d.mu.Lock()
	d.booting = false
	d.mu.Unlock()
}
