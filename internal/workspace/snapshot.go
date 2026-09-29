package workspace

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/github"
)

type SnapshotBuilder struct {
	github github.Provider
}

func NewSnapshotBuilder(
	githubProvider github.Provider,
) *SnapshotBuilder {
	return &SnapshotBuilder{
		github: githubProvider,
	}
}

func (b *SnapshotBuilder) Build(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) (*TempWorkspace, error) {
	archive, err := b.github.DownloadRepositoryArchive(
		ctx,
		installationID,
		organization,
		repository,
		ref,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"download repository archive: %w",
			err,
		)
	}

	ws, err := NewTempWorkspace()
	if err != nil {
		return nil, fmt.Errorf(
			"create temporary workspace: %w",
			err,
		)
	}

	if err := extractTarGz(archive, ws.Root()); err != nil {
		_ = ws.Close()

		return nil, fmt.Errorf(
			"extract repository archive: %w",
			err,
		)
	}

	if err := flattenArchiveRoot(ws.Root()); err != nil {
		_ = ws.Close()

		return nil, fmt.Errorf(
			"flatten repository archive: %w",
			err,
		)
	}

	return ws, nil
}

const (
	maxExtractedFileSize = 16 << 20  // 16 MiB
	maxExtractedSize     = 256 << 20 // 256 MiB
)

func extractTarGz(
	archive []byte,
	destination string,
) error {
	gzipReader, err := gzip.NewReader(
		bytes.NewReader(archive),
	)
	if err != nil {
		return fmt.Errorf(
			"create gzip reader: %w",
			err,
		)
	}

	tarReader := tar.NewReader(gzipReader)

	root, err := os.OpenRoot(destination)
	if err != nil {
		_ = gzipReader.Close()

		return fmt.Errorf(
			"open extraction root: %w",
			err,
		)
	}
	defer root.Close()

	var extractedSize int64

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf(
				"read tar archive: %w",
				err,
			)
		}

		archivePath, err := safeArchivePath(header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(
				archivePath,
				0700,
			); err != nil {
				return fmt.Errorf(
					"create directory %q: %w",
					header.Name,
					err,
				)
			}

		case tar.TypeReg:
			if header.Size < 0 {
				return fmt.Errorf(
					"invalid file size for %q",
					header.Name,
				)
			}

			if header.Size > maxExtractedFileSize {
				return fmt.Errorf(
					"file %q exceeds maximum extracted file size",
					header.Name,
				)
			}

			if extractedSize+header.Size > maxExtractedSize {
				return fmt.Errorf(
					"repository archive exceeds maximum extracted size",
				)
			}

			parent := filepath.Dir(archivePath)

			if parent != "." {
				if err := root.MkdirAll(
					parent,
					0700,
				); err != nil {
					return fmt.Errorf(
						"create parent directory for %q: %w",
						header.Name,
						err,
					)
				}
			}

			file, err := root.OpenFile(
				archivePath,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				0600,
			)
			if err != nil {
				return fmt.Errorf(
					"create archive file %q: %w",
					header.Name,
					err,
				)
			}

			written, copyErr := io.Copy(
				file,
				io.LimitReader(
					tarReader,
					maxExtractedFileSize+1,
				),
			)

			closeErr := file.Close()

			if copyErr != nil {
				return fmt.Errorf(
					"extract file %q: %w",
					header.Name,
					copyErr,
				)
			}

			if closeErr != nil {
				return fmt.Errorf(
					"close extracted file %q: %w",
					header.Name,
					closeErr,
				)
			}

			if written != header.Size {
				return fmt.Errorf(
					"unexpected extracted size for %q: expected %d, got %d",
					header.Name,
					header.Size,
					written,
				)
			}

			extractedSize += written
		}
	}

	if err := gzipReader.Close(); err != nil {
		return fmt.Errorf(
			"close gzip reader: %w",
			err,
		)
	}

	return nil
}

func safeArchivePath(name string) (string, error) {
	cleanName := filepath.Clean(name)

	if cleanName == "." ||
		cleanName == ".." ||
		strings.HasPrefix(
			cleanName,
			".."+string(os.PathSeparator),
		) ||
		filepath.IsAbs(cleanName) {
		return "", fmt.Errorf(
			"unsafe archive path: %s",
			name,
		)
	}

	return cleanName, nil
}

func flattenArchiveRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	archiveRoot := filepath.Join(
		root,
		entries[0].Name(),
	)

	children, err := os.ReadDir(archiveRoot)
	if err != nil {
		return err
	}

	for _, child := range children {
		oldPath := filepath.Join(
			archiveRoot,
			child.Name(),
		)

		newPath := filepath.Join(
			root,
			child.Name(),
		)

		if err := os.Rename(
			oldPath,
			newPath,
		); err != nil {
			return err
		}
	}

	return os.Remove(archiveRoot)
}
