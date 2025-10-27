package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/sdk/internal/testutil"
)

func BenchmarkKnowledgeBaseBuilderSimple(b *testing.B) {
	doc := writeTempDoc(b, "guide.md", "# Guide\n\nContent")
	setupCtx := testutil.NewBenchmarkContext(b)
	fileSource := mustBuildSource(b, setupCtx, NewFileSource(doc))
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := NewBase("kb-simple").WithDescription("Simple knowledge base").WithEmbedder("embedder-default").WithVectorDB("vectordb-default")
		builder.AddSource(fileSource)
		return builder.Build(ctx)
	})
}

func BenchmarkKnowledgeBaseBuilderMedium(b *testing.B) {
	root := b.TempDir()
	dir := filepath.Join(root, "docs")
	createTempDirWithDocs(b, dir, 3)
	setupCtx := testutil.NewBenchmarkContext(b)
	dirSource := mustBuildSource(b, setupCtx, NewDirectorySource(dir))
	urlSource := mustBuildSource(b, setupCtx, NewURLSource("https://example.com/one", "https://example.com/two"))
	apiSource := mustBuildSource(b, setupCtx, NewAPISource("confluence"))
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := NewBase("kb-medium").WithDescription("Medium knowledge base").WithEmbedder("embedder-team").WithVectorDB("vectordb-medium").WithChunking(ChunkStrategyRecursiveTextSplitter, 512, 64).WithRetrieval(10, 0.35, 2048).WithIngestMode(IngestModeOnStart).WithPreprocess(true, true)
		builder.AddSource(dirSource)
		builder.AddSource(urlSource)
		builder.AddSource(apiSource)
		return builder.Build(ctx)
	})
}

func BenchmarkKnowledgeBaseBuilderComplex(b *testing.B) {
	root := b.TempDir()
	irregular := filepath.Join(root, "knowledge")
	createTempDirWithDocs(b, irregular, 6)
	setupCtx := testutil.NewBenchmarkContext(b)
	directorySource := mustBuildSource(b, setupCtx, NewDirectorySource(irregular))
	fileSource := mustBuildSource(b, setupCtx, NewFileSource(writeTempDoc(b, "policy.md", "policy content")))
	urlSource := mustBuildSource(b, setupCtx, NewURLSource("https://docs.example.com/a", "https://docs.example.com/b"))
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := NewBase("kb-complex").WithDescription("Comprehensive knowledge base").WithEmbedder("embedder-pro").WithVectorDB("vectordb-pro").WithChunking(ChunkStrategyRecursiveTextSplitter, 384, 32).WithRetrieval(20, 0.25, 4096).WithIngestMode(IngestModeManual).WithPreprocess(true, false)
		builder.AddSource(directorySource)
		builder.AddSource(fileSource)
		builder.AddSource(urlSource)
		return builder.Build(ctx)
	})
}

func BenchmarkKnowledgeBaseBuilderParallel(b *testing.B) {
	doc := writeTempDoc(b, "faq.md", "Q&A")
	setupCtx := testutil.NewBenchmarkContext(b)
	source := mustBuildSource(b, setupCtx, NewFileSource(doc))
	testutil.RunParallelBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := NewBase("kb-parallel").WithEmbedder("embedder-parallel").WithVectorDB("vectordb-parallel").WithRetrieval(5, 0.4, 1024)
		builder.AddSource(source)
		return builder.Build(ctx)
	})
}

func mustBuildSource(b *testing.B, ctx context.Context, builder *SourceBuilder) *engineknowledge.SourceConfig {
	b.Helper()
	cfg, err := builder.Build(ctx)
	if err != nil {
		b.Fatalf("failed to build knowledge source: %v", err)
	}
	return cfg
}

func writeTempDoc(b *testing.B, name string, content string) string {
	b.Helper()
	path := filepath.Join(b.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("failed to write temp doc: %v", err)
	}
	return path
}

func createTempDirWithDocs(b *testing.B, dir string, files int) {
	b.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	for i := 0; i < files; i++ {
		path := filepath.Join(dir, fmt.Sprintf("doc-%d.md", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("# Document %d\nGenerated at %s", i, time.Now().Format(time.RFC3339Nano))), 0o600); err != nil {
			b.Fatalf("failed to write temp doc %d: %v", i, err)
		}
	}
}
