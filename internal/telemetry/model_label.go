package telemetry

import "sync/atomic"

// modelClasses is the Registries.Models of the export Init last started; nil classifies every model as unknown.
var modelClasses atomic.Pointer[func(provider, model string) ModelClass]

func setModelClasses(classify func(provider, model string) ModelClass) {
	if classify == nil {
		modelClasses.Store(nil)
		return
	}
	modelClasses.Store(&classify)
}

// ModelLabel is the gen_ai.request.model value a metric records: the catalog name, "custom", or "other". Metrics
// classify when they record, not when they export, so a series keeps one label for its lifetime: a cumulative series
// relabeled at export would look reset under its old label and double-counted under its new one.
func ModelLabel(provider, model string) string {
	if classify := modelClasses.Load(); classify != nil {
		switch (*classify)(provider, model) {
		case ModelCatalog:
			return model
		case ModelCustom:
			return ValueCustom
		}
	}
	return ValueOther
}
