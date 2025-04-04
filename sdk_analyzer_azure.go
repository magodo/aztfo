package main

import (
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"

	"github.com/magodo/aztfo/typeutils"
	"golang.org/x/tools/go/ssa"
)

type SDKAnalyzerAzure struct {
	pattern *regexp.Regexp
}

// NewSDKAnalyzerAzure builds a SDK analyzer for Azure Track1 SDK.
// The pattern specifies the regexp pattern of these SDK package paths.
func NewSDKAnalyzerAzure(pattern *regexp.Regexp) *SDKAnalyzerAzure {
	return &SDKAnalyzerAzure{
		pattern: pattern,
	}
}

func (a *SDKAnalyzerAzure) Name() string {
	return "Azure"
}

func (a *SDKAnalyzerAzure) FindSDKAPIFuncs(pkgs Packages) (map[*ssa.Function]APIOperation, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	prog := pkgs[0].ssa.Prog
	usedSdkMethods := usedSDKMethods(a, pkgs.Pkgs())

	// For each used SDK methods, try to find a method in the same receiver that is named after "Preparer" (as it contains
	// the information we are interested in).
	res := map[*ssa.Function]APIOperation{}
	for method := range usedSdkMethods {
		preparerMethod := method.MethodName + "Preparer"
		f := typeutils.NamedTypeMethodByName(method.Recv, preparerMethod)
		if f == nil {
			continue
		}

		prepareMethodDecl, err := typeutils.TypeFunc2DeclarationWithFile(method.File, f)
		if err != nil {
			return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), preparerMethod)
		}

		thisMethod := typeutils.NamedTypeMethodByName(method.Recv, method.MethodName)
		if thisMethod == nil {
			return nil, fmt.Errorf("failed to find the function type of %s.%s", method.Recv.Obj().Id(), method.MethodName)
		}
		thisMethodDecl, err := typeutils.TypeFunc2DeclarationWithFile(method.File, thisMethod)
		if err != nil {
			return nil, fmt.Errorf("failed to find the declaration of %s.%s", method.Recv.Obj().Id(), method.MethodName)
		}
		isLRO := isSDKFuncLRO(thisMethodDecl, method.Pkg, "FutureAPI")

		// Analyze the preparer function and gather the interested information.
		var (
			apiVersion string
			apiPath    string
			opKind     OperationKind
		)

		ast.Inspect(prepareMethodDecl.Body, func(node ast.Node) bool {
			switch node := node.(type) {
			// Looking for api version
			case *ast.DeclStmt:
				decl, ok := node.Decl.(*ast.GenDecl)
				if !ok {
					return false
				}
				if len(decl.Specs) != 1 {
					return false
				}
				vspec, ok := decl.Specs[0].(*ast.ValueSpec)
				if !ok {
					return false
				}
				if len(vspec.Names) != 1 || len(vspec.Values) != 1 {
					return false
				}
				name, value := vspec.Names[0], vspec.Values[0]
				if name.Name != "APIVersion" {
					return false
				}
				apiVersion, _ = strconv.Unquote(value.(*ast.BasicLit).Value)
				return false
			// Looking for api path and create operation kind
			case *ast.AssignStmt:
				lhs := node.Lhs
				if len(lhs) != 1 {
					return false
				}
				lIdent, ok := lhs[0].(*ast.Ident)
				if !ok {
					return false
				}
				if lIdent.Name != "preparer" {
					return false
				}
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
						pathLit, ok := callexpr.Args[0].(*ast.BasicLit)
						if !ok {
							continue
						}
						apiPath, _ = strconv.Unquote(pathLit.Value)
						apiPath = normalizeAPIPath(apiPath)
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
			return nil, fmt.Errorf("SDK operation info of the %s.%s is not complete: %s", method.Recv.Obj().Id(), preparerMethod, strings.Join(diags, ","))
		}

		ssaFunc := prog.LookupMethod(method.Recv, method.Pkg.Types, method.MethodName)
		if ssaFunc == nil {
			return nil, fmt.Errorf("failed to find the ssa function of %s.%s: %v", method.Recv.Obj().Id(), method.MethodName, err)
		}

		res[ssaFunc] = APIOperation{
			Kind:    opKind,
			Version: apiVersion,
			Path:    apiPath,
			IsLRO:   isLRO,
		}
	}

	return res, nil
}

func (a *SDKAnalyzerAzure) PackagePattern() *regexp.Regexp {
	return a.pattern
}
