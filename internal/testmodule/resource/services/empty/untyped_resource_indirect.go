package empty

import "github.com/magodo/aztfo/internal/testmodule/resource/pluginsdk"

func untypedResourceIndirect() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: untypedResourceIndirectCreate(),
		Read:   untypedResourceIndirectRead(),
		Update: untypedResourceIndirectUpdate(),
		Delete: untypedResourceIndirectDelete(),
	}
}

func untypedResourceIndirectCreate() pluginsdk.CreateFunc {
	return func(d *pluginsdk.ResourceData, meta interface{}) error {
		return nil
	}
}
func untypedResourceIndirectRead() pluginsdk.ReadFunc {
	return func(d *pluginsdk.ResourceData, meta interface{}) error {
		return nil
	}
}
func untypedResourceIndirectUpdate() pluginsdk.UpdateFunc {
	return func(d *pluginsdk.ResourceData, meta interface{}) error {
		return nil
	}
}
func untypedResourceIndirectDelete() pluginsdk.DeleteFunc {
	return func(d *pluginsdk.ResourceData, meta interface{}) error {
		return nil
	}
}
