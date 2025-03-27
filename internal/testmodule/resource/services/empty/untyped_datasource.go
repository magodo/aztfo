package empty

import "github.com/magodo/aztfp/internal/testmodule/resource/pluginsdk"

func untypedDataSource() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Read: untypedDataSourceRead,
	}
}

func untypedDataSourceRead(d *pluginsdk.ResourceData, meta interface{}) error {
	return nil
}
