package main

import (
	"go/types"

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

func resReachSDK(graph *callgraph.Graph, resFunc *ssa.Function, sdkFuncs map[*ssa.Function]APIOperation) []APIOperation {
	var reachableApiOps []APIOperation
	for tgtFunc, apiOp := range sdkFuncs {
		srcNode := graph.Nodes[resFunc]
		targetNode := graph.Nodes[tgtFunc]
		if targetNode == nil {
			continue
		}
		if callgraph.PathSearch(srcNode, func(n *callgraph.Node) bool { return n == targetNode }) != nil {
			reachableApiOps = append(reachableApiOps, apiOp)
		}
	}
	return reachableApiOps
}
