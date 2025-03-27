package sdk

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ResourceMetaData struct{}

type ResourceRunFunc func(ctx context.Context, metadata ResourceMetaData) error

type ResourceFunc struct {
	Func ResourceRunFunc
}

type resourceWithPluginSdkSchema interface {
	// Arguments is a list of user-configurable (that is: Required, Optional, or Optional and Computed)
	// arguments for this Resource
	Arguments() map[string]*schema.Schema

	// Attributes is a list of read-only (e.g. Computed-only) attributes
	Attributes() map[string]*schema.Schema
}

type resourceBase interface {
	resourceWithPluginSdkSchema
	ModelObject() interface{}
	ResourceType() string
}

type Resource interface {
	resourceBase

	Create() ResourceFunc
	Read() ResourceFunc
	Delete() ResourceFunc
}

type DataSource interface {
	resourceBase
	Read() ResourceFunc
}
