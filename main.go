package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

func main() {
	flagDir := flag.String("chdir", ".", "Switch to a different working directory")
	flag.Usage = func() {
		fmt.Println(`Usage: aztfp [options] <packages>

Arguments:
  - packages 
	The Go package pattern (default ".")

Options:`)
		flag.PrintDefaults()
	}
	flag.Parse()

	var patterns []string
	if len(flag.Args()) == 0 {
		patterns = append(patterns, ".")
	} else {
		patterns = append(patterns, flag.Args()...)
	}

	pkgs, graph, err := loadPackages(*flagDir, patterns...)
	if err != nil {
		log.Fatal(err)
	}

	// Find per resource information
	resources, err := findResources(pkgs)
	if err != nil {
		log.Fatal(err)
	}

	// Find sdk functions
	sdkFunctions, err := findSDKAPIFuncs(pkgs)
	if err != nil {
		log.Fatal(err)
	}

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
			printMsg(resId, "read", resReachSDK(graph, funcs.R, sdkFunctions))
		}
		if !resId.IsDataSource {
			if f := funcs.C; f != nil {
				printMsg(resId, "create", resReachSDK(graph, funcs.C, sdkFunctions))
			}
			if f := funcs.U; f != nil {
				printMsg(resId, "update", resReachSDK(graph, funcs.U, sdkFunctions))
			}
			if f := funcs.D; f != nil {
				printMsg(resId, "delete", resReachSDK(graph, funcs.D, sdkFunctions))
			}
		}
	}
}
