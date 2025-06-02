package ref

import "strings"

type WithRef struct {
	Use   string `json:"$use,omitempty"   yaml:"$use,omitempty"`
	Ref   string `json:"$ref,omitempty"   yaml:"$ref,omitempty"`
	Merge any    `json:"$merge,omitempty" yaml:"$merge,omitempty"`
}

func (w *WithRef) HasRef() bool {
	return w.Ref != ""
}

func (w *WithRef) HasUse() bool {
	return w.Use != ""
}

func (w *WithRef) HasMerge() bool {
	return w.Merge != nil
}

func (w *WithRef) HasDirective() bool {
	return w.HasMerge() || w.HasRef() || w.HasUse()
}

func (w *WithRef) isComponent(component string) bool {
	return w.HasUse() && strings.HasPrefix(w.Use, component)
}

func (w *WithRef) IsAgent() bool {
	return w.isComponent("agent(")
}

func (w *WithRef) IsTool() bool {
	return w.isComponent("tool(")
}

func (w *WithRef) IsTask() bool {
	return w.isComponent("task(")
}
