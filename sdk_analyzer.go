package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/magodo/aztfo/typeutils"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type OperationKind string

const (
	OperationKindGet     OperationKind = "GET"
	OperationKindPut                   = "PUT"
	OperationKindPost                  = "POST"
	OperationKindDelete                = "DELETE"
	OperationKindOptions               = "OPTIONS"
	OperationKindHead                  = "HEAD"
	OperationKindPatch                 = "PATCH"
)

type APIOperations []APIOperation

func (a APIOperations) Len() int {
	return len(a)
}

func (a APIOperations) Less(i int, j int) bool {
	x, y := a[i], a[j]
	if x.Path != y.Path {
		return x.Path < y.Path
	}
	if x.Version != y.Version {
		return x.Version < y.Version
	}
	if x.Kind != y.Kind {
		return x.Kind < y.Kind
	}
	if x.IsLRO != y.IsLRO {
		return x.IsLRO
	}
	return true
}

func (a *APIOperations) Union(b APIOperations) {
	for _, op := range b {
		if !slices.Contains(*a, op) {
			*a = append(*a, op)
		}
	}
}

func (a APIOperations) Swap(i int, j int) {
	a[i], a[j] = a[j], a[i]
}

type APIOperation struct {
	Kind    OperationKind `json:"kind"`
	Version string        `json:"version"`
	Path    string        `json:"path"`
	IsLRO   bool          `json:"is_lro"`
}

type SDKMethod struct {
	// The package that has the receiver (client) defined
	Pkg *packages.Package
	// The file that has the receiver (client) defined
	File       *ast.File
	Recv       *types.Named
	MethodName string
}

type SDKAnalyzer interface {
	// Name returns the SDK analyzer name
	Name() string

	// PackagePattern returns a compiled regexp which will be used to match the package import
	// path to tell whether the package belongs to the this SDK.
	PackagePattern() *regexp.Regexp

	// FindSDKAPIFuncs looks into the "pkgs" to find all the used Go SDK functions/methods that corresponds to an API operation.
	FindSDKAPIFuncs(pkgs Packages) (map[*ssa.Function]APIOperation, error)
}

// findSDKAPIFuncs finds the SDK API related functions defiend by the imported SDK packages from pkgs.
// The SDK can be either the Azure Track1 SDK or Hashicorp SDK.
func findSDKAPIFuncs(pkgs Packages) (map[*ssa.Function]APIOperation, error) {
	log.Println("Find SDK API functions: begin")
	defer log.Println("Find SDK API functions: end")

	sdkAnalyzers := []SDKAnalyzer{
		NewSDKAnalyzerAzure(
			regexp.MustCompile(
				`github.com/Azure/azure-sdk-for-go/services/(preview/)?[\w-]+/mgmt|` +
					`github.com/jackofallops/kermit/sdk/[\w-]+`,
			),
		),
		NewSDKAnalyzerHashicorp(
			regexp.MustCompile(`github.com/hashicorp/go-azure-sdk/resource-manager`),
			pkgs.Pkgs(),
		),
	}

	res := map[*ssa.Function]APIOperation{}
	for _, sdkanalyzer := range sdkAnalyzers {
		funcs, err := sdkanalyzer.FindSDKAPIFuncs(pkgs)
		if err != nil {
			return nil, err
		}
		maps.Copy(res, funcs)
	}
	return res, nil
}

// usedSDKMethods gathers all the SDK methods that the "pkgs" used.
// It basically finds all "SDK" packages imported by the "pkgs", iterate each of them, looking for
// method calls whose receiver is defined in "SDK" packages.
func usedSDKMethods(a SDKAnalyzer, pkgs []*packages.Package) map[SDKMethod]struct{} {
	sdkPkgMap := map[*packages.Package]struct{}{}

	// Filter the imported packages to only keep the SDK packages.
	for _, pkg := range pkgs {
		for _, epkg := range pkg.Imports {
			if !a.PackagePattern().MatchString(epkg.PkgPath) {
				continue
			}
			sdkPkgMap[epkg] = struct{}{}
		}
	}
	var sdkPkgs []*packages.Package
	for epkg := range sdkPkgMap {
		sdkPkgs = append(sdkPkgs, epkg)
	}

	usedSdkMethods := map[SDKMethod]struct{}{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				recvIdent, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}

				recvObj := pkg.TypesInfo.Uses[recvIdent]
				if !typeutils.IsUnderlyingNamedStruct(recvObj.Type()) {
					return true
				}
				recvType := typeutils.DereferenceR(recvObj.Type()).(*types.Named)

				// Ensure the receiver is defined in sdk packages
				recvTypePkg := recvType.Obj().Pkg()
				if recvTypePkg == nil {
					return true
				}
				if !a.PackagePattern().MatchString(recvTypePkg.Path()) {
					return true
				}

				pkg, file := typeutils.FindPos(sdkPkgs, recvType.Obj().Pos())
				if file == nil {
					panic(fmt.Sprintf("failed to find %q.%q in sdk packages", recvTypePkg.Path(), recvType.Obj().Id()))
				}

				m := SDKMethod{
					Pkg:        pkg,
					File:       file,
					Recv:       recvType,
					MethodName: sel.Sel.Name,
				}

				usedSdkMethods[m] = struct{}{}

				return true
			})
		}
	}
	return usedSdkMethods
}

func normalizeAPIPath(p string) string {
	segs := strings.Split(p, "/")
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			out = append(out, "{}")
			continue
		}
		out = append(out, strings.ToUpper(seg))
	}
	return strings.Join(out, "/")
}

func isSDKFuncLRO(fdecl *ast.FuncDecl, pkg *packages.Package, lroFieldName string) bool {
	if fdecl.Type.Results == nil || len(fdecl.Type.Results.List) == 0 {
		return false
	}

	if ident, ok := fdecl.Type.Results.List[0].Type.(*ast.Ident); ok {
		if obj := pkg.TypesInfo.ObjectOf(ident); obj != nil {
			if typeutils.IsUnderlyingNamedStruct(obj.Type()) {
				t := typeutils.DereferenceR(obj.Type()).(*types.Named).Underlying().(*types.Struct)
				if t.NumFields() > 0 {
					if t.Field(0).Name() == lroFieldName {
						return true
					}
				}
			}
		}
	}
	return false
}
