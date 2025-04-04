package empty

import (
	"context"

	"github.com/magodo/aztfo/internal/testmodule/resource/sdk"
)

var _ sdk.DataSource = TypedDataSource{}

type TypedDataSource struct{}

func (t TypedDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedDataSource) ResourceType() string {
	return "typed_datasource"
}
