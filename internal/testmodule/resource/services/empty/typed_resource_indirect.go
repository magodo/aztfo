package empty

import (
	"context"

	"github.com/magodo/aztfp/internal/testmodule/resource/sdk"
)

var _ sdk.Resource = TypedResourceIndirect{}

type TypedResourceIndirect struct{}

func (t TypedResourceIndirect) Create() sdk.ResourceFunc {
	return t.buildResourceFunc()
}

func (t TypedResourceIndirect) Read() sdk.ResourceFunc {
	return buildResourceFunc()
}

func (t TypedResourceIndirect) Update() sdk.ResourceFunc {
	return t.buildResourceFunc()
}

func (t TypedResourceIndirect) Delete() sdk.ResourceFunc {
	return buildResourceFunc()
}

func buildResourceFunc() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceIndirect) buildResourceFunc() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceIndirect) ResourceType() string {
	return "typed_resource_indirect"
}
