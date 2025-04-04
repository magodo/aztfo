package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"maps"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/magodo/aztfo/typeutils"
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
	Name         string `json:"name"`
	IsDataSource bool   `json:"is_data_source"`
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
		var regFound bool
		for _, regName := range []string{"Registration", "autoRegistration"} {
			reg := pkg.pkg.Types.Scope().Lookup(regName)
			if reg == nil {
				continue
			}
			regFound = true

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

		if !regFound {
			return nil, fmt.Errorf(`"Registration" not found at package %q`, pkg.pkg.PkgPath)
		}
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

	resourceInitFuncs := map[ResourceId]*types.Func{}

	// Mostly this function contains only a composite literal of resource map, e.g.
	//
	// 	resources := map[string]*pluginsdk.Resource{
	// 		"azurerm_management_lock": resourceManagementLock(),
	// 		"azurerm_resource_group":  resourceResourceGroup(),
	// 	}
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
				err = multierror.Append(err, fmt.Errorf("registration function object of %q not found", name))
				return false
			}
			resourceInitFuncs[ResourceId{Name: name, IsDataSource: isDataSource}] = f.(*types.Func)
		}

		return false
	})
	if err != nil {
		return nil, err
	}

	// There are also cases some resources will be conditionally registered, e.g.
	//
	// if !features.FivePointOh() {
	//     resources["azurerm_maps_creator"] = resourceMapsCreator()
	// }
	ast.Inspect(fdecl.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		idxExpr, ok := assign.Lhs[0].(*ast.IndexExpr)
		if !ok {
			return true
		}
		idx, ok := idxExpr.Index.(*ast.BasicLit)
		if !ok {
			return true
		}
		name, _ := strconv.Unquote(idx.Value)
		f := pkg.pkg.TypesInfo.ObjectOf(assign.Rhs[0].(*ast.CallExpr).Fun.(*ast.Ident))
		if f == nil {
			err = multierror.Append(err, fmt.Errorf("registration function object of %q not found", name))
			return false
		}
		resourceInitFuncs[ResourceId{Name: name, IsDataSource: isDataSource}] = f.(*types.Func)

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
						// Need to follow the call.
						ff, err := findResourceFunc(pkg.ssa.Prog, pkg, typeutils.SSAFunction(pkg.ssa, v.Fun.(*ast.Ident).Name))
						if err != nil {
							panic(fmt.Sprintf("failed to follow untyped resource func for %q: %v", rid, err))
						}
						return ff
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

			ssaf := prog.MethodValue(sel)
			if ssaf == nil {
				return nil, fmt.Errorf("failed to find the ssa function determined by %q", sel.String())
			}

			f, err := findResourceFunc(prog, pkg, ssaf)
			if err != nil {
				return nil, err
			}
			if f == nil {
				continue
			}

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

// findResourceFunc finds the resource func for both typed and untyped resources.
// For typed:
// findResourceFunc finds the sdk.ResourceRunFunc defined in the sdk.ResourceFunc, as an anonymous function, that is returned by the CRUD methods.
// If the CRUD methods indirect the call to another function, it will follow the call as long as it only contains a single return of the call.
// Example: Delete() of azurerm_resource_deployment_script_azure_power_shell
// For untyped:
// findResourceFunc finds the pluginsdk.XXXFunc, which is most of the time a simple function.
// If the CRUD methods indirect the call to another function, it will follow the call as long as it only contains a single return of the call.
// Example: Update() of azurerm_hdinsight_hbase_cluster
//
// In both case when indirect call occurs, there will be always a single annonymous function.
func findResourceFunc(prog *ssa.Program, pkg Package, ssaf *ssa.Function) (*ssa.Function, error) {
	switch len(ssaf.AnonFuncs) {
	case 0:
		return followResourceFunc(prog, pkg, ssaf.Object().(*types.Func))
	case 1:
		return ssaf.AnonFuncs[0], nil
	default:
		return nil, fmt.Errorf("unexpected anonymous function count for %q: got=%d", ssaf.String(), len(ssaf.AnonFuncs))
	}
}

func followResourceFunc(prog *ssa.Program, pkg Package, f *types.Func) (*ssa.Function, error) {
	fdecl, err := typeutils.TypeFunc2DeclarationWithPkg(pkg.pkg, f)
	if err != nil {
		return nil, fmt.Errorf("lookup function declaration from object of %q failed: %v", f.String(), err)
	}
	if l := len(fdecl.Body.List); l != 1 {
		return nil, fmt.Errorf("expect resource function body to contain only one statement, got=%d", l)
	}
	// Here we can assume the only statement is a return statement, which has only one result, per the signature of sdk.ResourceFunc.
	res := fdecl.Body.List[0].(*ast.ReturnStmt).Results[0]
	callexpr, ok := res.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("unexpected return value of resource function %q, expect to be a call expression", f.String())
	}

	switch fun := callexpr.Fun.(type) {
	case *ast.SelectorExpr:
		// Method call
		recv := pkg.pkg.TypesInfo.TypeOf(fun.X).(*types.Named)
		methodName := fun.Sel.Name
		ssaFunc := prog.LookupMethod(recv, pkg.pkg.Types, methodName)
		return findResourceFunc(prog, pkg, ssaFunc)
	case *ast.Ident:
		// Regular function call
		fobj := pkg.pkg.TypesInfo.ObjectOf(fun).(*types.Func)
		return findResourceFunc(prog, pkg, typeutils.SSAFunction(pkg.ssa, fobj.Name()))
	default:
		return nil, fmt.Errorf("unexpected returned call expression function type from %s: %T", f.String(), fun)
	}
}
