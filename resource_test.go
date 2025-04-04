package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindResources(t *testing.T) {
	t.Parallel()
	pkgs, _, err := loadPackages("./internal/testmodule/resource/services/empty", nil, []string{"."})
	require.NoError(t, err)

	infos, err := findResources(pkgs)
	require.NoError(t, err)

	require.Equal(t, 8, len(infos))

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
		info := infos[ResourceId{Name: "untyped_resource2", IsDataSource: false}]
		require.Equal(t, "untypedResourceCreate", info.C.Object().Name())
		require.Equal(t, "untypedResourceRead", info.R.Object().Name())
		require.Equal(t, "untypedResourceUpdate", info.U.Object().Name())
		require.Equal(t, "untypedResourceDelete", info.D.Object().Name())
	}
	{
		info := infos[ResourceId{Name: "untyped_resource_indirect", IsDataSource: false}]
		require.Equal(t, "untypedResourceIndirectCreate$1", info.C.Name())
		require.Equal(t, "untypedResourceIndirectRead$1", info.R.Name())
		require.Equal(t, "untypedResourceIndirectUpdate$1", info.U.Name())
		require.Equal(t, "untypedResourceIndirectDelete$1", info.D.Name())
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
	{
		info := infos[ResourceId{Name: "typed_resource_indirect", IsDataSource: false}]
		require.Equal(t, "(TypedResourceIndirect).buildResourceFunc$1", info.C.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "buildResourceFunc$1", info.R.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResourceIndirect).buildResourceFunc$1", info.U.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "buildResourceFunc$1", info.D.RelString(pkgs[0].pkg.Types))
	}
	{
		info := infos[ResourceId{Name: "typed_resource_gen", IsDataSource: false}]
		require.Equal(t, "(TypedResourceGen).Create$1", info.C.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResourceGen).Read$1", info.R.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResourceGen).Update$1", info.U.RelString(pkgs[0].pkg.Types))
		require.Equal(t, "(TypedResourceGen).Delete$1", info.D.RelString(pkgs[0].pkg.Types))
	}
}
