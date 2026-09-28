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
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

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

		target, err := safeArchivePath(
			destination,
			header.Name,
		)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(
				target,
				0755,
			); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(
				filepath.Dir(target),
				0755,
			); err != nil {
				return err
			}

			file, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				0644,
			)
			if err != nil {
				return err
			}

			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()

			if copyErr != nil {
				return copyErr
			}

			if closeErr != nil {
				return closeErr
			}
		}
	}

	return nil
}

func safeArchivePath(
	destination string,
	name string,
) (string, error) {
	cleanName := filepath.Clean(name)

	if cleanName == "." ||
		cleanName == ".." ||
		strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) ||
		filepath.IsAbs(cleanName) {
		return "", fmt.Errorf(
			"unsafe archive path: %s",
			name,
		)
	}

	target := filepath.Join(
		destination,
		cleanName,
	)

	return target, nil
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
