package start

import (
	"testing"

	"github.com/stretchr/testify/require"

	pkgconfig "github.com/compozy/compozy/pkg/config"
	"github.com/spf13/cobra"
)

// fakeService implements sourceGetter for tests.
type fakeService struct{ src pkgconfig.SourceType }

func (f fakeService) GetSource(_ string) pkgconfig.SourceType { return f.src }

func TestResolveStartMode(t *testing.T) {
	t.Run("Should accept --mode standalone", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("mode", "", "")
		_ = cmd.Flags().Set("mode", "standalone")
		got := resolveStartMode(cmd, fakeService{src: pkgconfig.SourceDefault}, "")
		require.Equal(t, "standalone", got)
	})

	t.Run("Should accept --mode distributed", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("mode", "", "")
		_ = cmd.Flags().Set("mode", "distributed")
		got := resolveStartMode(cmd, fakeService{src: pkgconfig.SourceDefault}, "")
		require.Equal(t, "distributed", got)
	})

	t.Run("Should prioritize config file over CLI flag", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("mode", "", "")
		_ = cmd.Flags().Set("mode", "standalone")
		// When source is YAML, do not override
		got := resolveStartMode(cmd, fakeService{src: pkgconfig.SourceYAML}, "distributed")
		require.Equal(t, "distributed", got)
	})

	t.Run("Should reject invalid --mode values via validation", func(t *testing.T) {
		// ensure invalid mode will fail config validation when applied
		cmd := &cobra.Command{}
		cmd.Flags().String("mode", "", "")
		_ = cmd.Flags().Set("mode", "bogus")
		got := resolveStartMode(cmd, fakeService{src: pkgconfig.SourceDefault}, "")
		cfg := pkgconfig.Default()
		cfg.Mode = got
		require.Error(t, pkgconfig.NewService().Validate(cfg))
	})
}
