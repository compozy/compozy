package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/resources"
)

func scanLoopResourceDir(ctx context.Context, root string, source looppkg.Source) ([]looppkg.ResourceSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("loop resource root %q is not a directory", root)
	}
	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if looppkg.IsDefinitionFileName(entry.Name()) {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Sort(files)

	specs := make([]looppkg.ResourceSpec, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		spec, _, err := looppkg.ParseResourceFile(file, looppkg.ResourceParseOptions{
			Source: source,
		})
		if err != nil {
			return nil, err
		}
		if filepath.Base(filepath.Dir(file)) != spec.Name {
			return nil, fmt.Errorf("loop file %q defines name %q", file, spec.Name)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func appendLoopResources(
	desired *[]loopPublicationInput,
	scope resources.ResourceScope,
	sourceKey string,
	specs []looppkg.ResourceSpec,
) {
	if desired == nil {
		return
	}
	for _, spec := range specs {
		*desired = append(*desired, loopPublicationInput{
			sourceKey: sourceKey + "/" + strings.TrimSpace(spec.Name),
			scope:     scope,
			spec:      looppkg.CloneResourceSpec(spec),
		})
	}
}

func loopSourceForDiscoveryRoot(source aghconfig.WorkspaceDiscoverySource) looppkg.Source {
	switch source {
	case aghconfig.WorkspaceDiscoverySourceWorkspace:
		return looppkg.SourceWorkspace
	case aghconfig.WorkspaceDiscoverySourceAdditional:
		return looppkg.SourceAdditional
	default:
		return looppkg.SourceUser
	}
}

func validateAndEncodeLoop(
	ctx context.Context,
	codec resources.KindCodec[looppkg.ResourceSpec],
	scope resources.ResourceScope,
	spec looppkg.ResourceSpec,
) (looppkg.ResourceSpec, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return looppkg.ResourceSpec{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return looppkg.ResourceSpec{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return looppkg.ResourceSpec{}, nil, err
	}
	return validated, canonical, nil
}
