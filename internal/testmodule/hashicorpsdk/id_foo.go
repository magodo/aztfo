package hashicorpsdk

import "fmt"

type FooId struct {
	SubscriptionId    string
	ResourceGroupName string
	FooName           string
}

func (id FooId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Foo/foos/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroupName, id.FooName)
}
