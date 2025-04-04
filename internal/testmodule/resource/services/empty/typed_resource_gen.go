package empty

import (
	"context"

	"github.com/magodo/aztfo/internal/testmodule/resource/sdk"
)

var _ sdk.Resource = TypedResourceGen{}

type TypedResourceGen struct{}

func (t TypedResourceGen) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceGen) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceGen) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceGen) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResourceGen) ResourceType() string {
	return "typed_resource_gen"
}
