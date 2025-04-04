package main

import (
	"context"

	"github.com/magodo/aztfo/internal/testmodule/azuresdk"
)

func main() {
	ctx := context.TODO()
	c := azuresdk.FooClient{}
	c.CreateOrUpdate(ctx, "", "", azuresdk.Foo{})
	c.Get(ctx, "", "")
}
