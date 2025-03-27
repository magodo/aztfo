package empty

import (
	"context"

	"github.com/magodo/aztfp/internal/testmodule/resource/sdk"
)

var _ sdk.DataSource = ComplexTypedDataSource{}

type ComplexTypedDataSource struct{}

func (t ComplexTypedDataSource) Read() sdk.ResourceFunc {
	return complexTypedDataSourceRead()
}

func (t ComplexTypedDataSource) ResourceType() string {
	return "complex_typed_datasource"
}

func complexTypedDataSourceRead() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}
