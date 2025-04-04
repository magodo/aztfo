package main

import (
	"context"

	"github.com/magodo/aztfo/internal/testmodule/hashicorpsdk"
)

func main() {
	ctx := context.TODO()
	c := hashicorpsdk.FooClientAutoRest{}
	c.UnlockDelete(ctx, hashicorpsdk.FooId{}, hashicorpsdk.FooRequest{})
}
