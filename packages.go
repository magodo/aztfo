package main

import (
	"errors"
	"fmt"
	"log"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

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

func loadPackages(dir string, pkgPathPrefixes []string, patterns []string) (Packages, *callgraph.Graph, error) {
	log.Println("Load packages: begin")
	defer log.Println("Load packages: end")

	// Loading Go packages
	cfg := packages.Config{Dir: dir, Mode: packages.LoadAllSyntax}
	pkgs, err := packages.Load(&cfg, patterns...)
	if err != nil {
		return nil, nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, nil, errors.New("packages contain errors")
	}

	// Build SSA for the specified "pkgs" and their dependencies.
	// The returned ssapkgs is the corresponding SSA Package of the specified "pkgs".
	prog, ssapkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	if len(ssapkgs) != len(pkgs) {
		panic(fmt.Sprintf("length of ssapkgs (%d) and pkgs (%d) are not equal", len(ssapkgs), len(pkgs)))
	}

	var packages Packages
	for i := range ssapkgs {
		packages = append(packages, Package{pkg: pkgs[i], ssa: ssapkgs[i]})
	}

	// Build callgraph
	graph := CallGraph(prog)

	if len(pkgPathPrefixes) != 0 {
		trimCallGraph(graph, pkgPathPrefixes)
	}

	return packages, graph, nil
}
