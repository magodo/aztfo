package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDKAnalyzerHashicorpAutoRest(t *testing.T) {
	t.Parallel()
	pkgs, _, err := loadPackages("./internal/testmodule/hashicorpsdkuser/autorest", ".")
	require.NoError(t, err)

	a := NewSDKAnalyzerHashicorp(regexp.MustCompile(`github.com/magodo/aztfp/internal/testmodule/hashicorpsdk`), pkgs.Pkgs())
	funcs, err := a.FindSDKAPIFuncs(pkgs)
	require.NoError(t, err)

	m := APIOperationMap{}
	for _, op := range funcs {
		m[op] = struct{}{}
	}
	require.Equal(t,
		[]APIOperation{
			{
				Kind:    OperationKindPost,
				Version: "2025-04-01",
				Path:    "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}/PROVIDERS/MICROSOFT.FOO/FOOS/{}/UNLOCKDELETE",
				IsLRO:   false,
			},
		},
		m.ToList())
}

func TestSDKAnalyzerHashicorpNative(t *testing.T) {
	t.Parallel()
	pkgs, _, err := loadPackages("./internal/testmodule/hashicorpsdkuser/native", ".")
	require.NoError(t, err)

	a := NewSDKAnalyzerHashicorp(regexp.MustCompile(`github.com/magodo/aztfp/internal/testmodule/hashicorpsdk`), pkgs.Pkgs())
	funcs, err := a.FindSDKAPIFuncs(pkgs)
	require.NoError(t, err)

	m := APIOperationMap{}
	for _, op := range funcs {
		m[op] = struct{}{}
	}
	require.Equal(t,
		[]APIOperation{
			{
				Kind:    OperationKindPut,
				Version: "2025-04-01",
				Path:    "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}/PROVIDERS/MICROSOFT.FOO/FOOS/{}",
				IsLRO:   true,
			},
		},
		m.ToList())
}
