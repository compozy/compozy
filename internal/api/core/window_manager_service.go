package core

type windowManagerStreamLifecycle = streamLifecycle

func newWindowManagerStreamLifecycle() *windowManagerStreamLifecycle {
	return newStreamLifecycle("window-manager")
}
