package daemon

import (
	"github.com/compozy/compozy/extensions/connectivity/tailscale"
	forgegithub "github.com/compozy/compozy/extensions/forge/github"
	opendesign "github.com/compozy/compozy/extensions/open-design"
	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

// bundledEnrollment enrolls one first-party built-in. Boot and restore both read bundledEnrollments,
// so their contents and order cannot drift apart.
type bundledEnrollment struct {
	Name   string
	Enroll func(compozyconfig.HomePaths, *extensionpkg.Registry) error
}

// bundledEnrollments lists the first-party built-ins in boot order. Boot walks the list in order.
var bundledEnrollments = []bundledEnrollment{
	{Name: opendesign.Name, Enroll: opendesign.EnsureManagedInstall},
	{Name: speccycle.Name, Enroll: speccycle.EnsureManagedInstall},
	{Name: tailscale.Name, Enroll: tailscale.EnsureManagedInstall},
	{Name: forgegithub.Name, Enroll: forgegithub.EnsureManagedInstall},
}
