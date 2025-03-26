package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"regexp"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/magodo/aztfp/typeutils"
	"golang.org/x/tools/go/ssa"
)

type ResourceInfos map[ResourceId]ResourceFuncs

type ResourceFuncs struct {
	C *ssa.Function
	R *ssa.Function
	U *ssa.Function
	D *ssa.Function
}

type ResourceId struct {
	Name         string
	IsDataSource bool
}

func findResources(pkgs []Package) (ResourceInfos, error) {
	servicePkgPattern := regexp.MustCompile(`github.com/hashicorp/terraform-provider-azurerm/internal/services/[\w-]+`)
	infos := ResourceInfos{}
	for _, pkg := range pkgs {
		if !servicePkgPattern.MatchString(pkg.pkg.PkgPath) {
			continue
		}

		reg := pkg.pkg.Types.Scope().Lookup("Registration")
		if reg == nil {
			return nil, fmt.Errorf(`"Registration" not found at package %q`, pkg.pkg.PkgPath)
		}

		var (
			methodSupportedDataSources *types.Func
			methodSupportedResources   *types.Func
			methodDataSources          *types.Func
			methodResources            *types.Func
		)
		for method := range reg.Type().(*types.Named).Methods() {
			switch method.Name() {
			case "SupportedDataSources":
				methodSupportedDataSources = method
			case "SupportedResources":
				methodSupportedResources = method
			case "DataSources":
				methodDataSources = method
			case "Resources":
				methodResources = method
			}
		}

		// fmt.Println(
		// 	methodSupportedDataSources.Name(),
		// 	methodSupportedResources.Name(),
		// 	methodDataSources.Name(),
		// 	methodResources.Name(),
		// )

		theInfos, err := findUnTypedResource(pkg, methodSupportedDataSources, true)
		if err != nil {
			return nil, fmt.Errorf("failed to find untyped data resources: %v", err)
		}
		maps.Copy(infos, theInfos)

		theInfos, err = findUnTypedResource(pkg, methodSupportedResources, false)
		if err != nil {
			return nil, fmt.Errorf("failed to find untyped resources: %v", err)
		}
		maps.Copy(infos, theInfos)

		theInfos, err = findTypedResource(pkg, methodDataSources, true)
		if err != nil {
			return nil, fmt.Errorf("failed to find typed data resources: %v", err)
		}
		maps.Copy(infos, theInfos)

		theInfos, err = findTypedResource(pkg, methodResources, false)
		if err != nil {
			return nil, fmt.Errorf("failed to find typed resources: %v", err)
		}
		maps.Copy(infos, theInfos)
	}

	return infos, nil
}

func findUnTypedResource(pkg Package, f *types.Func, isDataSource bool) (ResourceInfos, error) {
	fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, f)
	if err != nil {
		return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", f.Name(), err)
	}

	// Mostly this function contains only a composite literal of resource map, e.g.

	// 	resources := map[string]*pluginsdk.Resource{
	// 		"azurerm_management_lock": resourceManagementLock(),
	// 		"azurerm_resource_group":  resourceResourceGroup(),
	// 	}
	resourceInitFuncs := map[ResourceId]*types.Func{}
	ast.Inspect(fdecl.Body, func(n ast.Node) bool {
		complit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		mt, ok := complit.Type.(*ast.MapType)
		if !ok {
			return true
		}
		mtkey, ok := mt.Key.(*ast.Ident)
		if !ok {
			return true
		}
		if mtkey.Name != "string" {
			return true
		}
		mtval, ok := mt.Value.(*ast.StarExpr)
		if !ok {
			return true
		}
		mtvalsel, ok := mtval.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		mtvalselx, ok := mtvalsel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if mtvalselx.Name != "pluginsdk" {
			return true
		}
		if mtvalsel.Sel.Name != "Resource" {
			return true
		}

		for _, e := range complit.Elts {
			kv := e.(*ast.KeyValueExpr)
			name, _ := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
			f := pkg.pkg.TypesInfo.ObjectOf(kv.Value.(*ast.CallExpr).Fun.(*ast.Ident))
			if f == nil {
				err = multierror.Append(err, fmt.Errorf("function object of %q not found", name))
				return false
			}
			resourceInitFuncs[ResourceId{Name: name, IsDataSource: isDataSource}] = f.(*types.Func)
		}

		// TODO: Consider forms other than composite literal

		return false
	})

	if err != nil {
		return nil, err
	}

	// Find the CRUD functions from the resource init function
	infos := ResourceInfos{}
	for rid, initFunc := range resourceInitFuncs {
		// 	fmt.Printf("%v -> %s\n", rid, initFunc.Id())
		fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, initFunc)
		if err != nil {
			return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", initFunc.Name(), err)
		}

		funcs := ResourceFuncs{}
		ast.Inspect(fdecl.Body, func(n ast.Node) bool {
			complit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sl, ok := complit.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			slx, ok := sl.X.(*ast.Ident)
			if !ok {
				return true
			}
			if slx.Name != "pluginsdk" {
				return true
			}
			if sl.Sel.Name != "Resource" {
				return true
			}
			for _, e := range complit.Elts {
				kv := e.(*ast.KeyValueExpr)
				k, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch k.Name {
				case "Create":
					funcs.C = typeutils.SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Read":
					funcs.R = typeutils.SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Update":
					funcs.U = typeutils.SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Delete":
					funcs.D = typeutils.SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				}
			}
			return false

			// TODO: Consider forms other than composite literal
		})

		infos[rid] = funcs
	}

	return infos, nil
}

func findTypedResource(pkg Package, f *types.Func, isDataSource bool) (ResourceInfos, error) {
	return nil, nil
}
