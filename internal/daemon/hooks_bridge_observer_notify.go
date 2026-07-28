package daemon

import "context"

func notifyObserver[Observer any, Payload any](
	ctx context.Context,
	notifier *hooksNotifier,
	observer Observer,
	payload Payload,
	observerKind string,
	attributes []any,
	call func(context.Context, Observer, Payload) error,
) {
	if notifier == nil || any(observer) == nil || call == nil {
		return
	}
	notifyCtx, cancel := loopObserverContext(ctx)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			notifier.logger.Warn(
				"daemon: "+observerKind+" observer panic",
				appendObserverAttribute(attributes, "panic", recovered)...,
			)
		}
	}()
	if err := call(notifyCtx, observer, payload); err != nil {
		notifier.logger.Warn(
			"daemon: "+observerKind+" observer failed",
			appendObserverAttribute(attributes, "error", err)...,
		)
	}
}

func appendObserverAttribute(attributes []any, key string, value any) []any {
	result := make([]any, 0, len(attributes)+2)
	result = append(result, attributes...)
	return append(result, key, value)
}
