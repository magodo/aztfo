package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindResources(t *testing.T) {
	t.Parallel()
	pkgs, _, err := loadPackages("./internal/testmodule/resource/services/empty", ".")
	require.NoError(t, err)

	infos, err := findResources(pkgs)
	require.NoError(t, err)

	require.Equal(t, 5, len(infos))

	{
		info := infos[ResourceId{Name: "untyped_datasource", IsDataSource: true}]
		require.Nil(t, info.C)
		require.Nil(t, info.U)
		require.Nil(t, info.D)
		require.Equal(t, "untypedDataSourceRead", info.R.Object().Name())
	}
	{
		info := infos[ResourceId{Name: "untyped_resource", IsDataSource: false}]
		require.Equal(t, "untypedResourceCreate", info.C.Object().Name())
		require.Equal(t, "untypedResourceRead", info.R.Object().Name())
		require.Equal(t, "untypedResourceUpdate", info.U.Object().Name())
		require.Equal(t, "untypedResourceDelete", info.D.Object().Name())
	}
	{
		info := infos[ResourceId{Name: "typed_datasource", IsDataSource: true}]
		require.Nil(t, info.C)
		require.Nil(t, info.U)
		require.Nil(t, info.D)
		require.Equal(t, "(TypedDataSource).Read$1", info.R.RelString(pkgs[0].pkg.Types))
	}
	{
		info := infos[ResourceId{Name: "typed_resource", IsDataSource: false}]
		require.Equal(t, "(TypedResource).Create$1", info.C.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResource).Read$1", info.R.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResource).Update$1", info.U.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResource).Delete$1", info.D.RelString(pkgs[0].pkg.Types))
	}
}
