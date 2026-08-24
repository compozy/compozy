package daemon

import "context"

func (r *harnessLifecycleRecorder) SetProfileResolver(
	resolve func(context.Context, string) (string, error),
) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profileForSession = resolve
}
