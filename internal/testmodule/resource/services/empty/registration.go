package empty

import (
	"github.com/magodo/aztfo/internal/testmodule/resource/pluginsdk"
	"github.com/magodo/aztfo/internal/testmodule/resource/sdk"
)

type Registration struct{}

func (r Registration) SupportedDataSources() map[string]*pluginsdk.Resource {
	return map[string]*pluginsdk.Resource{
		"untyped_datasource": untypedDataSource(),
	}
}

// SupportedResources returns the supported Resources supported by this Service
func (r Registration) SupportedResources() map[string]*pluginsdk.Resource {
	resources := map[string]*pluginsdk.Resource{
		"untyped_resource":          untypedResource(),
		"untyped_resource_indirect": untypedResourceIndirect(),
	}
	if true {
		resources["untyped_resource2"] = untypedResource()
	}
	return resources
}

// DataSources returns a list of Data Sources supported by this Service
func (r Registration) DataSources() []sdk.DataSource {
	return []sdk.DataSource{
		TypedDataSource{},
	}
}

// Resources returns a list of Resources supported by this Service
func (r Registration) Resources() []sdk.Resource {
	return []sdk.Resource{
		TypedResource{},
		TypedResourceIndirect{},
	}
}
