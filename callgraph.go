package main

import (
	"go/types"
	"maps"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// This is the same as the static.CallGraph, except adding the anonymous functions to the graph (as source nodes).
func CallGraph(prog *ssa.Program) *callgraph.Graph {
	cg := callgraph.New(nil)

	// Recursively follow all static calls.
	seen := make(map[int]bool) // node IDs already seen

	var visitAnon func(fnode *callgraph.Node)
	var visit func(fnode *callgraph.Node)

	visitAnon = func(fnode *callgraph.Node) {
		// Anonymous functions can't be called as a static callee, i.e., it can onyl be a source node.
		// Hence no need to record seen nodes.
		for _, af := range fnode.Func.AnonFuncs {
			anode := cg.CreateNode(af)
			visit(anode)
		}
	}

	visit = func(fnode *callgraph.Node) {
		if !seen[fnode.ID] {
			seen[fnode.ID] = true

			for _, b := range fnode.Func.Blocks {
				for _, instr := range b.Instrs {
					if site, ok := instr.(ssa.CallInstruction); ok {
						if g := site.Common().StaticCallee(); g != nil {
							gnode := cg.CreateNode(g)
							callgraph.AddEdge(fnode, site, gnode)
							visit(gnode)
						}
					}
				}
			}

			visitAnon(fnode)
		}
	}

	methodsOf := func(T types.Type) {
		if !types.IsInterface(T) {
			mset := prog.MethodSets.MethodSet(T)
			for i := 0; i < mset.Len(); i++ {
				visit(cg.CreateNode(prog.MethodValue(mset.At(i))))
			}
		}
	}

	// Start from package-level symbols.
	for _, pkg := range prog.AllPackages() {
		for _, mem := range pkg.Members {
			switch mem := mem.(type) {
			case *ssa.Function:
				// package-level function
				visit(cg.CreateNode(mem))

			case *ssa.Type:
				// methods of package-level non-interface non-parameterized types
				if !types.IsInterface(mem.Type()) {
					if named, ok := mem.Type().(*types.Named); ok &&
						named.TypeParams() == nil {
						methodsOf(named)                   //  T
						methodsOf(types.NewPointer(named)) // *T
					}
				}
			}
		}
	}

	return cg
}

func trimCallGraph(graph *callgraph.Graph, pkgPathPrefixes []string) {
	//graph.DeleteSyntheticNodes() // This takes a lot of time...
	oldNodes := map[*ssa.Function]*callgraph.Node{}
	maps.Copy(oldNodes, graph.Nodes)
	for f, node := range oldNodes {
		if f == nil {
			continue
		}
		if f.Pkg == nil {
			continue
		}
		pkgPath := f.Pkg.Pkg.Path()

		var ok bool
		for _, prefix := range pkgPathPrefixes {
			if strings.HasPrefix(pkgPath, prefix) {
				ok = true
				break
			}
		}
		if ok {
			continue
		}

		graph.DeleteNode(node)
	}
}

type APIOperationMap map[APIOperation]struct{}

func (m APIOperationMap) ToList() APIOperations {
	l := APIOperations(slices.Collect(maps.Keys(m)))
	sort.Sort(l)
	return l
}

func resReachSDK(graph *callgraph.Graph, resFunc *ssa.Function, sdkFuncs map[*ssa.Function]APIOperation) []APIOperation {
	// Using a map to unify multiple ssa functions end up to be the same APIOperation.
	// E.g. A resource function can reach to DeleteThenPoll(), which in turns can reach to Delete(). Both corresponds to the same delete API operation.
	//      In this case, only this operation will be recorded as a result.
	m := APIOperationMap{}
	for tgtFunc, apiOp := range sdkFuncs {
		srcNode := graph.Nodes[resFunc]
		targetNode := graph.Nodes[tgtFunc]
		if targetNode == nil {
			continue
		}
		if callgraph.PathSearch(srcNode, func(n *callgraph.Node) bool { return n == targetNode }) != nil {
			m[apiOp] = struct{}{}
		}
	}
	return m.ToList()
}
