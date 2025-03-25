package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"maps"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const (
	pkgPathAzureRMProvider                = "github.com/hashicorp/terraform-provider-azurerm"
	pkgPathCommonSchema                   = "github.com/hashicorp/go-azure-helpers/resourcemanager"
	pkgPathAzureRMProviderPluginSDK       = "github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	pkgPathAzureRMProviderSDK             = "github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	pkgPathPluginSDKSchema                = "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	pkgPathPandoraAzureSDKResourceManager = "github.com/hashicorp/go-azure-sdk/resource-manager"
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

type Package struct {
	ssa *ssa.Package
	pkg *packages.Package
}

type Packages []Package

func (pkgs Packages) Pkgs() []*packages.Package {
	var pkgpkgs []*packages.Package
	for _, pkg := range pkgs {
		pkgpkgs = append(pkgpkgs, pkg.pkg)
	}
	return pkgpkgs
}

func main() {
	flag.Parse()
	var patterns []string
	if len(flag.Args()) == 0 {
		patterns = append(patterns, ".")
	} else {
		patterns = append(patterns, flag.Args()...)
	}

	pkgs, err := loadPackages(patterns...)
	if err != nil {
		log.Fatal(err)
	}

	// Find per resource information
	resources, err := findResources(pkgs)
	if err != nil {
		log.Fatal(err)
	}
	_ = resources

	// Find sdk functions
	sdkFunctions, err := findSDKFuncs(pkgs)
	if err != nil {
		log.Fatal(err)
	}
	_ = sdkFunctions

	printMsg := func(resourceId ResourceId, method string, apiOps []APIOperation) {
		apiOpsMsgs := []string{}
		for _, op := range apiOps {
			msg := fmt.Sprintf("%s %s %s", op.Kind, op.Path, op.Version)
			if op.IsLRO {
				msg += " (LRO)"
			}
			apiOpsMsgs = append(apiOpsMsgs, msg)
		}
		resourceName := resourceId.Name
		if resourceId.IsDataSource {
			resourceName += "(DS)"
		}
		fmt.Printf("%s - %s\n%s\n===\n", resourceName, method, strings.Join(apiOpsMsgs, "\n"))
	}

	// For each resource method, find the reachable SDK functions, using RTA analysis.
	for resId, funcs := range resources {
		if f := funcs.R; f != nil {
			printMsg(resId, "read", resReachSDK(funcs.R, sdkFunctions))
		}
		if !resId.IsDataSource {
			if f := funcs.C; f != nil {
				printMsg(resId, "create", resReachSDK(funcs.C, sdkFunctions))
			}
			if f := funcs.U; f != nil {
				printMsg(resId, "update", resReachSDK(funcs.U, sdkFunctions))
			}
			if f := funcs.D; f != nil {
				printMsg(resId, "delete", resReachSDK(funcs.D, sdkFunctions))
			}
		}
	}
}

func resReachSDK(f *ssa.Function, sdkFuncs map[*ssa.Function]APIOperation) []APIOperation {
	var reachableApiOps []APIOperation
	result := rta.Analyze([]*ssa.Function{f}, true)
	for rf := range result.Reachable {
		if apiOp, ok := sdkFuncs[rf]; ok {
			// Explicitly check the path from f -> rf, to exclude the "exported methods of runtime types (since they may be accessed via reflection)" accounted by ".Reachable".
			src, dst := result.CallGraph.Nodes[f], result.CallGraph.Nodes[rf]
			if len(callGraphFindPath(src, dst, make(map[*callgraph.Node]bool), nil)) != 0 {
				reachableApiOps = append(reachableApiOps, apiOp)
			}
		}
	}
	return reachableApiOps
}

func callGraphFindPath(src, dst *callgraph.Node, visited map[*callgraph.Node]bool, path []*callgraph.Node) []*callgraph.Node {
	if src == dst {
		return append(path, src)
	}
	if visited[src] {
		return nil
	}
	visited[src] = true
	path = append(path, src)

	for _, edge := range src.Out {
		nextNode := edge.Callee
		if result := callGraphFindPath(nextNode, dst, visited, path); result != nil {
			return result
		}
	}

	return nil
}

func loadPackages(patterns ...string) ([]Package, error) {
	// Loading Go packages
	cfg := packages.Config{Dir: ".", Mode: packages.LoadAllSyntax}
	pkgs, err := packages.Load(&cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("packages contain errors")
	}

	// Build SSA for the specified "pkgs" and their dependencies.
	// The returned ssapkgs is the corresponding SSA Package of the specified "pkgs".
	prog, ssapkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	if len(ssapkgs) != len(pkgs) {
		panic(fmt.Sprintf("length of ssapkgs (%d) and pkgs (%d) are not equal", len(ssapkgs), len(pkgs)))
	}

	var packages []Package
	for i := range ssapkgs {
		packages = append(packages, Package{pkg: pkgs[i], ssa: ssapkgs[i]})
	}

	return packages, nil
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
	fdecl, err := TypeFunc2DeclarationWithPkg(pkg.pkg, f)
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
		fdecl, err := TypeFunc2DeclarationWithPkg(pkg.pkg, initFunc)
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
					funcs.C = SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Read":
					funcs.R = SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Update":
					funcs.U = SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
				case "Delete":
					funcs.D = SSAFunction(pkg.ssa, kv.Value.(*ast.Ident).Name)
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

func findSDKFuncs(pkgs Packages) (map[*ssa.Function]APIOperation, error) {
	sdkAnalyzers := []SDKAnalyzer{
		NewSDKAnalyzerTrack1(),
		NewSDKAnalyzerPandora(),
	}

	res := map[*ssa.Function]APIOperation{}
	for _, sdkanalyzer := range sdkAnalyzers {
		funcs, err := sdkanalyzer.FindSDKFuncs(pkgs)
		if err != nil {
			return nil, err
		}
		maps.Copy(res, funcs)
	}
	return res, nil
}

// Utils

func TypeFunc2DeclarationWithFile(file *ast.File, f *types.Func) (*ast.FuncDecl, error) {
	pos := f.Pos()
	// Lookup the function declaration from the method identifier position.
	// The returned enclosing interval starts from the identifier node, then the function declaration node.
	paths, _ := astutil.PathEnclosingInterval(file, pos, pos)
	fdecl := paths[1].(*ast.FuncDecl)
	return fdecl, nil
}
func TypeFunc2DeclarationWithPkg(pkg *packages.Package, f *types.Func) (*ast.FuncDecl, error) {
	return TypeFunc2DeclarationWithPkgs([]*packages.Package{pkg}, f)
}

func TypeFunc2DeclarationWithPkgs(pkgs []*packages.Package, f *types.Func) (*ast.FuncDecl, error) {
	_, file := FindPos(pkgs, f.Pos())
	if file == nil {
		return nil, fmt.Errorf("function %q doesn't belong to any file in the specified packages", f.Name())
	}
	return TypeFunc2DeclarationWithFile(file, f)
}

func FindPos(pkgs []*packages.Package, pos token.Pos) (*packages.Package, *ast.File) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			if pos >= file.FileStart && pos <= file.FileEnd {
				return pkg, file
			}
		}
	}
	return nil, nil
}

func SSAFunction(pkg *ssa.Package, funcName string) *ssa.Function {
	return pkg.Func(funcName)
}

func SSAMethod(pkg *ssa.Package, tpkg *types.Package, recv types.Type, methodName string) *ssa.Function {
	return pkg.Prog.LookupMethod(recv, tpkg, methodName)
}
