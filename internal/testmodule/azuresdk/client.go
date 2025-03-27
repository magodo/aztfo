package azuresdk

import (
	"context"
	"net/http"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

type FooClient struct {
	BaseURI        string
	SubscriptionID string
}

func (client FooClient) Get(ctx context.Context, resourceGroupName string, fooName string) (result Foo, err error) {
	req, err := client.GetPreparer(ctx, resourceGroupName, fooName)
	if err != nil {
		return
	}

	resp, err := client.GetSender(req)
	if err != nil {
		return
	}

	result, err = client.GetResponder(resp)
	if err != nil {
		return
	}

	return
}

func (client FooClient) GetPreparer(ctx context.Context, resourceGroupName string, fooName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"fooName":           autorest.Encode("path", fooName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2025-04-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Foo/foos/{fooName}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

func (client FooClient) GetSender(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func (client FooClient) GetResponder(resp *http.Response) (result Foo, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

func (client FooClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, fooName string, foo Foo) (result FooCreateOrUpdateFuture, err error) {
	req, err := client.CreateOrUpdatePreparer(ctx, resourceGroupName, fooName, foo)
	if err != nil {
		return
	}

	result, err = client.CreateOrUpdateSender(req)
	if err != nil {
		return
	}

	return
}

func (client FooClient) CreateOrUpdatePreparer(ctx context.Context, resourceGroupName string, fooName string, foo Foo) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"fooName":           autorest.Encode("path", fooName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2025-04-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}
	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPut(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Foo/foos/{fooName}", pathParameters),
		autorest.WithJSON(foo),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

func (client FooClient) CreateOrUpdateSender(req *http.Request) (future FooCreateOrUpdateFuture, err error) {
	return FooCreateOrUpdateFuture{}, nil
}
