package empty

import "github.com/magodo/aztfp/internal/testmodule/resource/pluginsdk"

func untypedResource() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: untypedResourceCreate,
		Read:   untypedResourceRead,
		Update: untypedResourceUpdate,
		Delete: untypedResourceDelete,
	}
}

func untypedResourceCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	return nil
}
func untypedResourceRead(d *pluginsdk.ResourceData, meta interface{}) error {
	return nil
}
func untypedResourceUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	return nil
}
func untypedResourceDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	return nil
}
