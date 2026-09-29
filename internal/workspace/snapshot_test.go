package workspace

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/github"
)

func TestSnapshotBuilder_Build(t *testing.T) {
	archive := createTestArchive(t)

	fakeGitHub := &github.FakeProvider{
		Archive: archive,
	}

	builder := NewSnapshotBuilder(fakeGitHub)

	ws, err := builder.Build(
		context.Background(),
		123,
		"MorningBlossom",
		"ai-code-review",
		"head-456",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer ws.Close()

	goModPath := filepath.Join(ws.Root(), "go.mod")

	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	if string(goMod) != "module example.com/test\n" {
		t.Fatalf(
			"unexpected go.mod content: %q",
			string(goMod),
		)
	}

	retryPath := filepath.Join(
		ws.Root(),
		"payment",
		"retry.go",
	)

	retrySource, err := os.ReadFile(retryPath)
	if err != nil {
		t.Fatalf("read payment/retry.go: %v", err)
	}

	if string(retrySource) != "package payment\n" {
		t.Fatalf(
			"unexpected source content: %q",
			string(retrySource),
		)
	}
}

func createTestArchive(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer

	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	files := map[string]string{
		"repo-head/go.mod":           "module example.com/test\n",
		"repo-head/payment/retry.go": "package payment\n",
	}

	for path, content := range files {
		err := tarWriter.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0600,
			Size: int64(len(content)),
		})
		if err != nil {
			t.Fatalf("write tar header: %v", err)
		}

		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buffer.Bytes()
}
