package empty

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/magodo/aztfp/internal/testmodule/resource/sdk"
)

var _ sdk.Resource = TypedResource{}

type TypedResource struct{}

func (t TypedResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (t TypedResource) Arguments() map[string]*schema.Schema {
	return nil
}

func (t TypedResource) Attributes() map[string]*schema.Schema {
	return nil
}

func (t TypedResource) ModelObject() interface{} {
	return nil
}

func (t TypedResource) ResourceType() string {
	return "typed_resource"
}
