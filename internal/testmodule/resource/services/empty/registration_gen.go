package empty

import (
	"github.com/magodo/aztfp/internal/testmodule/resource/sdk"
)

type autoRegistration struct{}

// DataSources returns a list of Data Sources supported by this Service
func (r autoRegistration) DataSources() []sdk.DataSource {
	return []sdk.DataSource{}
}

// Resources returns a list of Resources supported by this Service
func (r autoRegistration) Resources() []sdk.Resource {
	return []sdk.Resource{
		TypedResourceGen{},
	}
}
