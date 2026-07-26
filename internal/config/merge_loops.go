package config

type loopsOverlay struct {
	Defaults loopsDefaultsOverlay `toml:"defaults"`
}

type loopsDefaultsOverlay struct {
	Delivery loopDefaultOverlay `toml:"delivery"`
	Watch    loopDefaultOverlay `toml:"watch"`
}

type loopDefaultOverlay struct {
	IterationCap  *int                         `toml:"iteration_cap"`
	NoProgress    loopNoProgressDefaultOverlay `toml:"no_progress"`
	Gates         loopGatesDefaultOverlay      `toml:"gates"`
	Budget        loopBudgetDefaultOverlay     `toml:"budget"`
	ModelDefaults loopModelDefaultsOverlay     `toml:"model_defaults"`
	FanOutWidth   *int                         `toml:"fan_out_width"`
}

type loopNoProgressDefaultOverlay struct {
	Window *int `toml:"window"`
}

type loopGatesDefaultOverlay struct {
	MaxRevisions *int `toml:"max_revisions"`
}

type loopBudgetDefaultOverlay struct {
	Tokens       *int    `toml:"tokens"`
	WallClockSec *int    `toml:"wall_clock_sec"`
	OnExceeded   *string `toml:"on_exceeded"`
}

type loopModelDefaultsOverlay struct {
	Worker *string `toml:"worker"`
	Judge  *string `toml:"judge"`
}

func (o loopsOverlay) Apply(dst *LoopsConfig) {
	o.Defaults.Apply(&dst.Defaults)
}

func (o loopsDefaultsOverlay) Apply(dst *LoopsDefaultsConfig) {
	o.Delivery.Apply(&dst.Delivery)
	o.Watch.Apply(&dst.Watch)
}

func (o loopDefaultOverlay) Apply(dst *LoopDefaultConfig) {
	if o.IterationCap != nil {
		dst.IterationCap = *o.IterationCap
	}
	o.NoProgress.Apply(&dst.NoProgress)
	o.Gates.Apply(&dst.Gates)
	o.Budget.Apply(&dst.Budget)
	o.ModelDefaults.Apply(&dst.ModelDefaults)
	if o.FanOutWidth != nil {
		dst.FanOutWidth = *o.FanOutWidth
	}
}

func (o loopNoProgressDefaultOverlay) Apply(dst *LoopNoProgressDefaultConfig) {
	if o.Window != nil {
		dst.Window = *o.Window
	}
}

func (o loopGatesDefaultOverlay) Apply(dst *LoopGatesDefaultConfig) {
	if o.MaxRevisions != nil {
		dst.MaxRevisions = *o.MaxRevisions
	}
}

func (o loopBudgetDefaultOverlay) Apply(dst *LoopBudgetDefaultConfig) {
	if o.Tokens != nil {
		dst.Tokens = *o.Tokens
	}
	if o.WallClockSec != nil {
		dst.WallClockSec = *o.WallClockSec
	}
	if o.OnExceeded != nil {
		dst.OnExceeded = *o.OnExceeded
	}
}

func (o loopModelDefaultsOverlay) Apply(dst *LoopModelDefaultsConfig) {
	if o.Worker != nil {
		dst.Worker = *o.Worker
	}
	if o.Judge != nil {
		dst.Judge = *o.Judge
	}
}
