package hashicorpsdk

import (
	"net/http"

	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
)

// AutoRest

type FooRequest struct{}
type FooResponse struct{}

type OperationResponse struct {
	HttpResponse *http.Response
	Model        *FooResponse
}

// Native

type Foo struct{}

type NativeOperationResponse struct {
	Poller       pollers.Poller
	HttpResponse *http.Response
	Model        *Foo
}
