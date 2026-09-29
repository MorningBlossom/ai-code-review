package workspace

import "context"

type Builder interface {
	Build(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		ref string,
	) (*TempWorkspace, error)
}
