package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/magodo/aztfp/typeutils"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type SDKAnalyzerHashicorp struct {
	pattern      *regexp.Regexp
	commonidsPkg *packages.Package
}

// NewSDKAnalyzerHashicorp builds a SDK analyzer for Hashicorp SDK.
// The pattern specifies the regexp pattern of the SDK package path.
func NewSDKAnalyzerHashicorp(pattern *regexp.Regexp, pkgs []*packages.Package) *SDKAnalyzerHashicorp {
	// "hashicorp/go-azure-helpers/resourcemanager/commonids" package is needed for constructing *some* resource ids, which is used later by this analyzer.
	var commonidsPkg *packages.Package
out:
	for _, pkg := range pkgs {
		for _, epkg := range pkg.Imports {
			if epkg.PkgPath == "github.com/hashicorp/go-azure-helpers/resourcemanager/commonids" {
				commonidsPkg = epkg
				break out
			}
		}
	}
	return &SDKAnalyzerHashicorp{
		pattern:      pattern,
		commonidsPkg: commonidsPkg,
	}
}

func (a *SDKAnalyzerHashicorp) Name() string {
	return "Hashicorp"
}

func (a *SDKAnalyzerHashicorp) FindSDKAPIFuncs(pkgs Packages) (map[*ssa.Function]APIOperation, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	prog := pkgs[0].ssa.Prog
	usedSdkMethods := usedSDKMethods(a, pkgs.Pkgs())

	res := map[*ssa.Function]APIOperation{}
	for method := range usedSdkMethods {
		var (
			apiOp *APIOperation
			err   error
		)
		if strings.HasSuffix(method.Pkg.Fset.Position(method.File.Pos()).Filename, "autorest.go") {
			apiOp, err = a.findSDKOperationForMethodAutoRest(method)
			if err != nil {
				return nil, fmt.Errorf("failed to find SDK operation (autorest) for method %s.%s: %v", method.Recv.Obj().Id(), method.MethodName, err)
			}
		} else {
			apiOp, err = a.findSDKOperationForMethodNative(method)
			if err != nil {
				return nil, fmt.Errorf("failed to find SDK operation (native) for method %s.%s: %v", method.Recv.Obj().Id(), method.MethodName, err)
			}
		}
		if apiOp == nil {
			continue
		}

		ssaFunc := prog.LookupMethod(method.Recv, method.Pkg.Types, method.MethodName)
		if ssaFunc == nil {
			return nil, fmt.Errorf("failed to find the ssa function of %s.%s: %v", method.Recv.Obj().Id(), method.MethodName, err)
		}

		res[ssaFunc] = *apiOp
	}

	return res, nil
}

// findSDKOperationForMethodAutoRest finds the autorest transport based method on the same receiver of the used SDK method, named after "preparerFor".
// If not found, returns nil APIOperation.
func (a *SDKAnalyzerHashicorp) findSDKOperationForMethodAutoRest(method SDKMethod) (*APIOperation, error) {
	preparerMethod := "preparerFor" + strings.TrimSuffix(method.MethodName, "ThenPoll")

	prepareFunc := typeutils.NamedTypeMethodByName(method.Recv, preparerMethod)
	if prepareFunc == nil {
		return nil, nil
	}

	prepareFuncDecl, err := typeutils.TypeFunc2DeclarationWithPkg(method.Pkg, prepareFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), preparerMethod)
	}

	thisMethod := typeutils.NamedTypeMethodByName(method.Recv, method.MethodName)
	if thisMethod == nil {
		return nil, nil
	}
	thisMethodDecl, err := typeutils.TypeFunc2DeclarationWithPkg(method.Pkg, thisMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), method.MethodName)
	}
	isLRO := isSDKFuncLRO(thisMethodDecl, method.Pkg, "Poller")

	// Analyze the preparer function and gather the interested information.
	var (
		apiVersion string
		apiPath    string
		opKind     OperationKind
	)

	ast.Inspect(prepareFuncDecl.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		// Looking for api path, version and operation kind
		case *ast.AssignStmt:
			lhs := node.Lhs
			if len(lhs) != 1 {
				return false
			}
			lIdent, ok := lhs[0].(*ast.Ident)
			if !ok {
				return false
			}

			switch lIdent.Name {
			// API version
			case "queryParameters":
				kvs := node.Rhs[0].(*ast.CompositeLit).Elts
				for _, kv := range kvs {
					kv := kv.(*ast.KeyValueExpr)
					k, ok := kv.Key.(*ast.BasicLit)
					if !ok {
						continue
					}
					klit, _ := strconv.Unquote(k.Value)
					if klit != "api-version" {
						continue
					}
					v, ok := kv.Value.(*ast.Ident)
					if !ok {
						continue
					}
					apiVersionObj, ok := method.Pkg.TypesInfo.Uses[v]
					if !ok {
						continue
					}
					apiVersion = constant.StringVal(apiVersionObj.(*types.Const).Val())
				}
				return false
			// API Path and Operation kind
			case "preparer":
				for _, arg := range node.Rhs[0].(*ast.CallExpr).Args {
					callexpr, ok := arg.(*ast.CallExpr)
					if !ok {
						continue
					}
					fun, ok := callexpr.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					switch fun.Sel.Name {
					case "WithPathParameters",
						"WithPath":
						firstArgCallExpr, ok := callexpr.Args[0].(*ast.CallExpr)
						if !ok {
							continue
						}
						sel, ok := firstArgCallExpr.Fun.(*ast.SelectorExpr)
						if !ok {
							continue
						}
						switch sel.X.(*ast.Ident).Name {
						// Call the id.ID() to construct the api path
						case "id":
							apiPath, ok = a.apiPathFromID(method.Pkg, sel)
							// Call the fmt.Sprintf() to construct the api path
						case "fmt":
							// e.g. '"%s/eventhubs/"'
							formatString, _ := strconv.Unquote(firstArgCallExpr.Args[0].(*ast.BasicLit).Value)
							sel := firstArgCallExpr.Args[1].(*ast.CallExpr).Fun.(*ast.SelectorExpr)
							apiPath, ok = a.apiPathFromID(method.Pkg, sel)
							apiPath = normalizeAPIPath(fmt.Sprintf(formatString, apiPath))
						default:
							panic(fmt.Sprintf("unexpected WithPath/WithPathParameters call happened at %s", method.Pkg.Fset.Position(callexpr.Pos())))
						}
					case "AsGet":
						opKind = OperationKindGet
					case "AsPut":
						opKind = OperationKindPut
					case "AsPost":
						opKind = OperationKindPost
					case "AsDelete":
						opKind = OperationKindDelete
					case "AsOption":
						opKind = OperationKindOptions
					case "AsHead":
						opKind = OperationKindHead
					case "AsPatch":
						opKind = OperationKindPatch
					default:
						continue
					}
				}
				return false

			default:
				return false
			}
		default:
			return true
		}
	})
	// Some API (e.g. track1 resources/resources.go) can accept the APIVersion as a parameter.
	if apiVersion == "" {
		apiVersion = "unknown"
	}

	var diags []string
	if apiPath == "" {
		diags = append(diags, "api path is not found")
	}
	if opKind == "" {
		diags = append(diags, "API operation kind is not found")
	}
	if len(diags) != 0 {
		return nil, fmt.Errorf("SDK operation info of the %s.%s is not complete: %s", method.Recv.Obj().Id(), method.MethodName,
			strings.Join(diags, ","))
	}

	return &APIOperation{
		Kind:    opKind,
		Version: apiVersion,
		Path:    apiPath,
		IsLRO:   isLRO,
	}, nil
}

// findSDKOperationForMethodNative finds the native transport based method on the same receiver of the used SDK method.
// If not found, returns nil APIOperation.
func (a *SDKAnalyzerHashicorp) findSDKOperationForMethodNative(method SDKMethod) (*APIOperation, error) {
	methodName := method.MethodName
	if strings.HasSuffix(methodName, "ThenPoll") {
		// PUT/DELETE
		methodName = strings.TrimSuffix(methodName, "ThenPoll")
	} else if strings.HasSuffix(methodName, "CompleteMatchingPredicate") {
		// "LIST"
		methodName = strings.TrimSuffix(methodName, "CompleteMatchingPredicate")
	} else if strings.HasSuffix(methodName, "Complete") {
		// "LIST"
		methodName = strings.TrimSuffix(methodName, "Complete")
	}

	sdkFunc := typeutils.NamedTypeMethodByName(method.Recv, methodName)
	if sdkFunc == nil {
		return nil, nil
	}

	sdkFuncDecl, err := typeutils.TypeFunc2DeclarationWithPkg(method.Pkg, sdkFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), methodName)
	}

	thisMethod := typeutils.NamedTypeMethodByName(method.Recv, method.MethodName)
	if thisMethod == nil {
		return nil, nil
	}
	thisMethodDecl, err := typeutils.TypeFunc2DeclarationWithPkg(method.Pkg, thisMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), method.MethodName)
	}
	isLRO := isSDKFuncLRO(thisMethodDecl, method.Pkg, "Poller")

	// Analyze the preparer function and gather the interested information.
	var (
		apiVersion string
		apiPath    string
		opKind     OperationKind
	)

	for _, f := range method.Pkg.Syntax {
		if filepath.Base(method.Pkg.Fset.Position(f.Package).Filename) == "version.go" {
			for _, decl := range f.Decls {
				decl, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				if len(decl.Specs) != 1 {
					continue
				}
				spec, ok := decl.Specs[0].(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(spec.Names) != 1 {
					continue
				}
				if spec.Names[0].Name != "defaultApiVersion" {
					continue
				}
				if len(spec.Values) != 1 {
					continue
				}
				lit, ok := spec.Values[0].(*ast.BasicLit)
				if !ok {
					continue
				}
				apiVersion, _ = strconv.Unquote(lit.Value)
				break
			}
		}
	}

	stmt, ok := sdkFuncDecl.Body.List[0].(*ast.AssignStmt)
	if !ok {
		return nil, nil
	}
	if len(stmt.Rhs) != 1 {
		return nil, nil
	}
	comp, ok := stmt.Rhs[0].(*ast.CompositeLit)
	if !ok {
		return nil, nil
	}
	compType, ok := comp.Type.(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	if v, ok := compType.X.(*ast.Ident); !ok || v.Name != "client" {
		return nil, nil
	}
	if compType.Sel.Name != "RequestOptions" {
		return nil, nil
	}
	for _, expr := range comp.Elts {
		expr := expr.(*ast.KeyValueExpr)
		exprKey, exprVal := expr.Key, expr.Value
		ident, ok := exprKey.(*ast.Ident)
		if !ok {
			continue
		}
		switch ident.Name {
		// opKind
		case "HttpMethod":
			switch exprVal.(*ast.SelectorExpr).Sel.Name {
			case "MethodGet":
				opKind = OperationKindGet
			case "MethodPost":
				opKind = OperationKindPost
			case "MethodPut":
				opKind = OperationKindPut
			case "MethodDelete":
				opKind = OperationKindDelete
			case "MethodHead":
				opKind = OperationKindHead
			case "MethodPatch":
				opKind = OperationKindPatch
			}
		case "Path":
			pathCall, ok := exprVal.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := pathCall.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			switch sel.X.(*ast.Ident).Name {
			// Call the id.ID() to construct the api path
			case "id":
				apiPath, ok = a.apiPathFromID(method.Pkg, sel)
				// Call the fmt.Sprintf() to construct the api path
			case "fmt":
				// e.g. '"%s/eventhubs/"'
				formatString, _ := strconv.Unquote(pathCall.Args[0].(*ast.BasicLit).Value)
				sel := pathCall.Args[1].(*ast.CallExpr).Fun.(*ast.SelectorExpr)
				apiPath, ok = a.apiPathFromID(method.Pkg, sel)
				apiPath = normalizeAPIPath(fmt.Sprintf(formatString, apiPath))
			default:
				panic(fmt.Sprintf("unexpected Path value call happened at %s", method.Pkg.Fset.Position(pathCall.Pos())))
			}
		}
	}

	// Some API (e.g. track1 resources/resources.go) can accept the APIVersion as a parameter.
	if apiVersion == "" {
		apiVersion = "unknown"
	}

	var diags []string
	if apiPath == "" {
		diags = append(diags, "api path is not found")
	}
	if opKind == "" {
		diags = append(diags, "API operation kind is not found")
	}
	if len(diags) != 0 {
		return nil, fmt.Errorf("SDK operation info of the %s.%s is not complete: %s", method.Recv.Obj().Id(), method.MethodName,
			strings.Join(diags, ","))
	}

	return &APIOperation{
		Kind:    opKind,
		Version: apiVersion,
		Path:    apiPath,
		IsLRO:   isLRO,
	}, nil
}

func (a *SDKAnalyzerHashicorp) PackagePattern() *regexp.Regexp {
	return a.pattern
}

func (a SDKAnalyzerHashicorp) apiPathFromID(sdkpkg *packages.Package, idSelExpr *ast.SelectorExpr) (string, bool) {
	idObj, ok := sdkpkg.TypesInfo.Uses[idSelExpr.X.(*ast.Ident)]
	if !ok {
		return "", false
	}

	idFunc := typeutils.NamedTypeMethodByName(idObj.Type().(*types.Named), idSelExpr.Sel.Name)
	if idFunc == nil {
		return "", false
	}

	// The id.ID() can be defined either in the same sdk package, or the commonids package.
	idPkgs := []*packages.Package{sdkpkg}
	if a.commonidsPkg != nil {
		idPkgs = append(idPkgs, a.commonidsPkg)
	}
	idFuncDecl, err := typeutils.TypeFunc2DeclarationWithPkgs(idPkgs, idFunc)
	if err != nil {
		panic(fmt.Sprintf("failed to find the declaration of %s.%s", idObj.Id(), idFunc.Name()))
	}

	apiPath, _ := strconv.Unquote(idFuncDecl.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.BasicLit).Value)
	apiPath = strings.ReplaceAll(apiPath, "%s", "{}")
	apiPath = normalizeAPIPath(apiPath)
	return apiPath, true
}
