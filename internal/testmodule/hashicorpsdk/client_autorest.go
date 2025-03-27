package hashicorpsdk

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

type FooClientAutoRest struct {
	Client  autorest.Client
	baseUri string
}

func (c FooClientAutoRest) UnlockDelete(ctx context.Context, id FooId, input FooRequest) (result OperationResponse, err error) {
	req, err := c.preparerForUnlockDelete(ctx, id, input)
	if err != nil {
		return
	}

	result.HttpResponse, err = c.Client.Send(req, azure.DoRetryWithRegistration(c.Client))
	if err != nil {
		return
	}

	result, err = c.responderForUnlockDelete(result.HttpResponse)
	if err != nil {
		return
	}

	return
}

func (c FooClientAutoRest) preparerForUnlockDelete(ctx context.Context, id FooId, input FooRequest) (*http.Request, error) {
	queryParameters := map[string]interface{}{
		"api-version": defaultApiVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPost(),
		autorest.WithBaseURL(c.baseUri),
		autorest.WithPath(fmt.Sprintf("%s/unlockDelete", id.ID())),
		autorest.WithJSON(input),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

func (c FooClientAutoRest) responderForUnlockDelete(resp *http.Response) (result OperationResponse, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result.Model),
		autorest.ByClosing())
	result.HttpResponse = resp

	return
}
