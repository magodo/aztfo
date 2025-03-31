package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"maps"
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

func (id ResourceId) String() string {
	ret := id.Name
	if id.IsDataSource {
		ret += " (DS)"
	}
	return ret
}

// findResources finds terraform resource (untyped+typed) information among the specified packages.
func findResources(pkgs []Package) (ResourceInfos, error) {
	log.Println("Find resources: begin")
	defer log.Println("Find resources: end")

	infos := ResourceInfos{}
	for _, pkg := range pkgs {
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
	if f == nil {
		return nil, nil
	}
	fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, f)
	if err != nil {
		return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", f.Id(), err)
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
		fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, initFunc)
		if err != nil {
			return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", initFunc.Id(), err)
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

				ssaFunc := func(v ast.Expr) *ssa.Function {
					switch v := v.(type) {
					case *ast.Ident:
						return typeutils.SSAFunction(pkg.ssa, v.Name)
					case *ast.CallExpr:
						// E.g. in "resourceHDInsightKafkaCluster":
						//
						// Update: hdinsightClusterUpdate("Kafka", resourceHDInsightKafkaClusterRead),
						//
						// This will then miss the Read() function call inside the Update operation, as we are only following static calls.
						return typeutils.SSAFunction(pkg.ssa, v.Fun.(*ast.Ident).Name)
					default:
						panic("unreachable")
					}
				}
				switch k.Name {
				case "Create":
					funcs.C = ssaFunc(kv.Value)
				case "Read":
					funcs.R = ssaFunc(kv.Value)
				case "Update":
					funcs.U = ssaFunc(kv.Value)
				case "Delete":
					funcs.D = ssaFunc(kv.Value)
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
	if f == nil {
		return nil, nil
	}
	fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, f)
	if err != nil {
		return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", f.Id(), err)
	}

	// Mostly this function contains only a composite literal of resource map, e.g.

	// return []sdk.Resource{
	// 	VirtualMachineImplicitDataDiskFromSourceResource{},
	// 	VirtualMachineRunCommandResource{},
	// 	GalleryApplicationResource{},
	//  ...
	//  }
	resourceTypes := []*types.Named{}
	ast.Inspect(fdecl.Body, func(n ast.Node) bool {
		complit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		at, ok := complit.Type.(*ast.ArrayType)
		if !ok {
			return true
		}
		ate, ok := at.Elt.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ateselx, ok := ate.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ateselx.Name != "sdk" {
			return true
		}
		if isDataSource {
			if ate.Sel.Name != "DataSource" {
				return true
			}
		} else {
			if ate.Sel.Name != "Resource" {
				return true
			}
		}

		for _, e := range complit.Elts {
			complit := e.(*ast.CompositeLit)
			t := pkg.pkg.TypesInfo.ObjectOf(complit.Type.(*ast.Ident)).Type().(*types.Named)
			resourceTypes = append(resourceTypes, t)
		}

		// TODO: Consider forms other than composite literal

		return false
	})

	infos := ResourceInfos{}
	for _, rt := range resourceTypes {
		// Retrieve the resource type
		resourceTypeFunc := typeutils.NamedTypeMethodByName(rt, "ResourceType")
		resourceTypeFuncDecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, resourceTypeFunc)
		if err != nil {
			return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", resourceTypeFunc.Id(), err)
		}

		var name string
		switch res := resourceTypeFuncDecl.Body.List[0].(*ast.ReturnStmt).Results[0].(type) {
		case *ast.BasicLit:
			name, _ = strconv.Unquote(res.Value)
		case *ast.Ident:
			name = res.Obj.Decl.(*ast.ValueSpec).Values[0].(*ast.BasicLit).Value
			name, _ = strconv.Unquote(name)
		default:
			panic("unreachable")
		}

		// Retrieve the methods
		prog := pkg.ssa.Prog
		funcs := ResourceFuncs{}
		for _, methodName := range []string{"Create", "Update", "Read", "Delete"} {
			sel := prog.MethodSets.MethodSet(rt).Lookup(pkg.pkg.Types, methodName)
			if sel == nil {
				continue
			}
			method := prog.MethodValue(sel)
			if method == nil {
				continue
			}
			if len(method.AnonFuncs) != 1 {
				// return nil, fmt.Errorf("expect one anonymous function directly beneth %s.%s, got=%d", rt.Obj().Id(), methodName, len(method.AnonFuncs))
				log.Printf("expect one anonymous function directly beneth %s.%s, got=%d\n", rt.Obj().Id(), methodName, len(method.AnonFuncs))
				continue
			}
			f := method.AnonFuncs[0]
			switch methodName {
			case "Create":
				funcs.C = f
			case "Update":
				funcs.U = f
			case "Read":
				funcs.R = f
			case "Delete":
				funcs.D = f
			}
		}

		infos[ResourceId{Name: name, IsDataSource: isDataSource}] = funcs
	}

	return infos, nil
}
