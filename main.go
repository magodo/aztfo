package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
)

func main() {
	flagDir := flag.String("chdir", ".", "terraform-provider-azurerm root directory")
	flagDebug := flag.Bool("debug", false, "Enable debug log")
	flag.Usage = func() {
		fmt.Println(`Usage: aztfp [options] <packages>

Arguments:
  - packages 
	The Go package pattern (default "./internal/...").
	Note that this is fixed in most of the time as it is a whole program analysis.

Options:`)
		flag.PrintDefaults()
	}
	flag.Parse()

	var patterns []string
	if len(flag.Args()) == 0 {
		patterns = append(patterns, "./internal/...")
	} else {
		patterns = append(patterns, flag.Args()...)
	}

	if !*flagDebug {
		log.SetOutput(io.Discard)
	}

	pkgPathPrefixes := []string{
		"github.com/hashicorp/terraform-provider-azurerm",
		"github.com/hashicorp/go-azure-sdk",
		"github.com/Azure/azure-sdk-for-go",
	}
	pkgs, graph, err := loadPackages(*flagDir, pkgPathPrefixes, patterns)
	if err != nil {
		log.Fatal(err)
	}

	// Find per resource information
	var servicePkgs Packages
	servicePkgPattern := regexp.MustCompile(`^github.com/hashicorp/terraform-provider-azurerm/internal/services/[\w-]+$`)
	for _, pkg := range pkgs {
		if servicePkgPattern.MatchString(pkg.pkg.PkgPath) {
			servicePkgs = append(servicePkgs, pkg)
		}
	}
	resources, err := findResources(servicePkgs)
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

	// For each resource method, find the reachable SDK functions, using static analysis.
	total := len(resources)
	n := 0
	for resId, funcs := range resources {
		n += 1
		log.Printf("[%d/%d] Reachability check for %q: begin\n", n, total, resId)

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

		log.Printf("[%d/%d] Reachability check for %q: end\n", n, total, resId)
	}
}
