package typeutils

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// DereferenceR returns a pointer's element type; otherwise it returns
// T. If the element type is itself a pointer, DereferenceR will be
// applied recursively.
func DereferenceR(T types.Type) types.Type {
	if p, ok := T.Underlying().(*types.Pointer); ok {
		return DereferenceR(p.Elem())
	}
	return T
}

func IsUnderlyingNamedStruct(t types.Type) bool {
	t = DereferenceR(t)
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	if _, ok := nt.Underlying().(*types.Struct); !ok {
		return false
	}
	return true
}

func NamedTypeMethodByName(t *types.Named, methodName string) *types.Func {
	for m := range t.Methods() {
		if m.Name() != methodName {
			continue
		}
		return m
	}
	return nil
}

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
