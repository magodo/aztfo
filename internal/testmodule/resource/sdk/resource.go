package sdk

import (
	"context"
)

type ResourceMetaData struct{}

type ResourceRunFunc func(ctx context.Context, metadata ResourceMetaData) error

type ResourceFunc struct {
	Func ResourceRunFunc
}

type resourceBase interface {
	ResourceType() string
}

type Resource interface {
	resourceBase

	Create() ResourceFunc
	Read() ResourceFunc
	Delete() ResourceFunc
}

type DataSource interface {
	resourceBase
	Read() ResourceFunc
}
