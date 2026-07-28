package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkMultiRegistrySearch(b *testing.B) {
	b.ReportAllocs()

	listingsA := benchmarkListings("alpha", 512, 128)
	listingsB := benchmarkListings("beta", 512, 128)
	listingsC := benchmarkListings("gamma", 512, 128)

	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "alpha",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return listingsA, nil
			},
		},
		&stubRegistrySource{
			name: "beta",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return listingsB, nil
			},
		},
		&stubRegistrySource{
			name: "gamma",
			caps: SourceCaps{Search: true},
			searchFunc: func(context.Context, string, SearchOpts) ([]Listing, error) {
				return listingsC, nil
			},
		},
	)

	ctx := context.Background()
	opts := SearchOpts{Limit: 100}

	for b.Loop() {
		listings, err := registry.Search(ctx, "prompt", opts)
		if err != nil {
			b.Fatalf("Search() error = %v", err)
		}
		if len(listings) == 0 {
			b.Fatal("Search() returned no listings")
		}
	}
}

func BenchmarkMultiRegistryResolveSource(b *testing.B) {
	b.ReportAllocs()

	registry := NewMultiRegistry(
		testLogger(),
		&stubRegistrySource{
			name: "low",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "shared", Name: "low", Version: "1.0.0"}}, nil
			},
		},
		&stubRegistrySource{
			name: "mid",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return nil, nil
			},
		},
		&stubRegistrySource{
			name: "high",
			infoFunc: func(context.Context, string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: "shared", Name: "high", Version: "2.0.0"}}, nil
			},
		},
	)

	ctx := context.Background()

	for b.Loop() {
		source, detail, err := registry.resolveSource(ctx, "shared")
		if err != nil {
			b.Fatalf("resolveSource() error = %v", err)
		}
		if source == nil || detail == nil {
			b.Fatal("resolveSource() returned nil result")
		}
	}
}

func BenchmarkExtractArchive(b *testing.B) {
	b.ReportAllocs()

	archive := mustBenchmarkTarGz(b, benchmarkArchiveEntries(48, 2048))
	baseRoot := b.TempDir()

	for i := 0; b.Loop(); i++ {
		destRoot := filepath.Join(baseRoot, strconv.Itoa(i))
		if err := ExtractArchive(bytes.NewReader(archive), destRoot); err != nil {
			b.Fatalf("ExtractArchive() error = %v", err)
		}
	}
}

func BenchmarkComputeInstallChecksum(b *testing.B) {
	b.ReportAllocs()

	root := benchmarkChecksumTree(b)

	for b.Loop() {
		checksum, err := computeInstallChecksum(root)
		if err != nil {
			b.Fatalf("computeInstallChecksum() error = %v", err)
		}
		if checksum == "" {
			b.Fatal("computeInstallChecksum() returned empty checksum")
		}
	}
}

func benchmarkListings(source string, count int, shared int) []Listing {
	if count < shared {
		shared = count
	}

	listings := make([]Listing, 0, count)
	for i := 0; i < shared; i++ {
		listings = append(listings, Listing{
			Slug:        fmt.Sprintf("shared-%03d", i),
			Name:        fmt.Sprintf("%s-shared-%03d", source, i),
			Description: strings.Repeat("shared description ", 2),
			Version:     fmt.Sprintf("1.%d.0", i%10),
		})
	}
	for i := shared; i < count; i++ {
		listings = append(listings, Listing{
			Slug:        fmt.Sprintf("%s-only-%03d", source, i-shared),
			Name:        fmt.Sprintf("%s-only-%03d", source, i-shared),
			Description: strings.Repeat("unique description ", 2),
			Version:     fmt.Sprintf("2.%d.0", i%10),
		})
	}
	return listings
}

func benchmarkArchiveEntries(count int, payloadSize int) []tarEntry {
	entries := make([]tarEntry, 0, count+1)
	entries = append(entries, tarEntry{name: "skill", typeflag: tar.TypeDir})
	for i := range count {
		entries = append(entries, tarEntry{
			name:    filepath.ToSlash(filepath.Join("skill", fmt.Sprintf("file-%03d.txt", i))),
			content: strings.Repeat(fmt.Sprintf("payload-%03d-", i), payloadSize/12+1)[:payloadSize],
		})
	}
	return entries
}

func benchmarkChecksumTree(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	for dirIndex := range 8 {
		dir := filepath.Join(root, fmt.Sprintf("pkg-%02d", dirIndex))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
		for fileIndex := range 32 {
			path := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", fileIndex))
			payload := strings.Repeat(fmt.Sprintf("payload-%02d-%02d-", dirIndex, fileIndex), 64)
			if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
				b.Fatalf("WriteFile(%q) error = %v", path, err)
			}
		}
	}
	if err := os.Symlink("pkg-00/file-00.txt", filepath.Join(root, "current")); err != nil {
		b.Fatalf("Symlink() error = %v", err)
	}
	return root
}

func mustBenchmarkTarGz(b *testing.B, entries []tarEntry) []byte {
	b.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     int64(len(entry.content)),
			Typeflag: entry.typeflag,
			Linkname: entry.linkname,
		}
		if header.Mode == 0 {
			if entry.typeflag == tar.TypeDir {
				header.Mode = 0o755
			} else {
				header.Mode = 0o644
			}
		}
		if header.Typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}
		if header.Typeflag == tar.TypeDir {
			header.Size = 0
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			b.Fatalf("WriteHeader(%q) error = %v", entry.name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.content)); err != nil {
				b.Fatalf("Write(%q) error = %v", entry.name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		b.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		b.Fatalf("gzipWriter.Close() error = %v", err)
	}

	return buffer.Bytes()
}
