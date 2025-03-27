package empty

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/magodo/aztfp/internal/testmodule/resource/sdk"
)

var _ sdk.DataSource = TypedDataSource{}

type TypedDataSource struct{}

func (t TypedDataSource) Arguments() map[string]*schema.Schema {
	return nil
}

func (t TypedDataSource) Attributes() map[string]*schema.Schema {
	return nil
}

func (t TypedDataSource) ModelObject() interface{} {
	return nil
}

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
