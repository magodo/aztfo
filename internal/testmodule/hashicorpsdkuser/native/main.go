package main

import (
	"context"

	"github.com/magodo/aztfo/internal/testmodule/hashicorpsdk"
)

func main() {
	ctx := context.TODO()
	c := hashicorpsdk.FooClientNative{}
	c.CreateThenPoll(ctx, hashicorpsdk.FooId{}, hashicorpsdk.Foo{})

	// Calling the non-LRO version won't cause a duplicated record
	c.Create(ctx, hashicorpsdk.FooId{}, hashicorpsdk.Foo{})
}
