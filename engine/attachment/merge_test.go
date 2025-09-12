package attachment

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func Test_ComputeEffectiveItems_DedupAndPrecedence(t *testing.T) {
	t.Run("Should de-duplicate by canonical key and apply action precedence", func(t *testing.T) {
		taskAtt := &ImageAttachment{
			baseAttachment: baseAttachment{MetaMap: map[string]any{"who": "task"}},
			Source:         SourceURL,
			URL:            "https://example.com/a.png",
		}
		agentAtt := &ImageAttachment{
			baseAttachment: baseAttachment{MetaMap: map[string]any{"who": "agent"}},
			Source:         SourceURL,
			URL:            "https://example.com/a.png",
		}
		actionAtt := &ImageAttachment{
			baseAttachment: baseAttachment{MetaMap: map[string]any{"who": "action"}},
			Source:         SourceURL,
			URL:            "https://example.com/a.png",
		}
		items := ComputeEffectiveItems(
			[]Attachment{taskAtt},
			nil,
			[]Attachment{agentAtt},
			nil,
			[]Attachment{actionAtt},
			nil,
		)
		require.Equal(t, 1, len(items))
		require.Equal(t, "action", items[0].Att.Meta()["who"])
	})
}

func Test_ComputeEffectiveItems_PathCanonicalization(t *testing.T) {
	t.Run("Should treat different relative forms as same file and keep latest", func(t *testing.T) {
		dir := t.TempDir()
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		// Same target file referenced differently
		taskAtt := &ImageAttachment{Source: SourcePath, Path: "img.png"}
		agentAtt := &ImageAttachment{Source: SourcePath, Path: "./img.png"}
		items := ComputeEffectiveItems([]Attachment{taskAtt}, cwd, []Attachment{agentAtt}, cwd, nil, nil)
		require.Equal(t, 1, len(items))
		// The last inserted with same key should win (agentAtt)
		_, src, _, p := attFields(items[0].Att)
		require.Equal(t, SourcePath, src)
		require.Equal(t, "./img.png", p)
	})
}
