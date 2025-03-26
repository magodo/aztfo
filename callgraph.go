package main

import (
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

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
