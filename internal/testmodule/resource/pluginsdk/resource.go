package pluginsdk

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type (
	Resource     = schema.Resource
	ResourceData = schema.ResourceData
)

type (
	CreateFunc = schema.CreateFunc
	DeleteFunc = schema.DeleteFunc
	ExistsFunc = schema.ExistsFunc
	ReadFunc   = schema.ReadFunc
	UpdateFunc = schema.UpdateFunc
)
