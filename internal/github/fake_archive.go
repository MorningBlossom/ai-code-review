package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
)

func NewFakeRepositoryArchive() ([]byte, error) {
	var buffer bytes.Buffer

	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	files := map[string]string{
		"test-repository-head123/go.mod": `module example.com/test-repository

go 1.27
`,
		"test-repository-head123/payment/retry.go": `package payment

func retryPayment() error {
	return nil
}
`,
	}

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write tar header: %w", err)
		}

		if _, err := tarWriter.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("write tar content: %w", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	return buffer.Bytes(), nil
}
