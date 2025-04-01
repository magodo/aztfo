package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDKAnalyzerAzure(t *testing.T) {
	t.Parallel()
	pkgs, _, err := loadPackages("./internal/testmodule/azuresdkuser", nil, []string{"."})
	require.NoError(t, err)

	a := NewSDKAnalyzerAzure(regexp.MustCompile(`github.com/magodo/aztfp/internal/testmodule/azuresdk`))
	funcs, err := a.FindSDKAPIFuncs(pkgs)
	require.NoError(t, err)

	m := APIOperationMap{}
	for _, op := range funcs {
		m[op] = struct{}{}
	}
	require.Equal(t,
		[]APIOperation{
			{
				Kind:    OperationKindGet,
				Version: "2025-04-01",
				Path:    "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}/PROVIDERS/MICROSOFT.FOO/FOOS/{}",
				IsLRO:   false,
			},
			{
				Kind:    OperationKindPut,
				Version: "2025-04-01",
				Path:    "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}/PROVIDERS/MICROSOFT.FOO/FOOS/{}",
				IsLRO:   true,
			},
		},
		m.ToList())
}
