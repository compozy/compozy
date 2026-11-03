// Package driftconst exercises the output schema drift check's constant reader.
package driftconst

// Kind is a named string type declared in every constant form Go allows.
type Kind string

const (
	Explicit  Kind = "explicit"
	Converted      = Kind("converted")
	Joined    Kind = "jo" + "ined"
	Aliased        = Explicit
)

// Untyped and Other must not count as Kind values.
const Untyped = "untyped"

// Other is a distinct named type sharing the package.
type Other string

const OtherValue Other = "other"
