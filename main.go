package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"regexp"
	"runtime"
	"sort"

	"github.com/magodo/workerpool"
)

func main() {
	flagDir := flag.String("chdir", ".", "terraform-provider-azurerm root directory")
	flagDebug := flag.Bool("debug", false, "Enable debug log")
	flag.Usage = func() {
		fmt.Println(`Usage: aztfo [options] <packages>

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

	// For each resource method, find the reachable SDK functions, using static analysis.
	var results Results
	wp := workerpool.NewWorkPool(runtime.NumCPU())
	n := 0
	total := len(resources)
	wp.Run(func(res any) error {
		n += 1
		result := res.(Result)
		log.Printf("[%d/%d] Reachability check for %q done\n", n, total, result.Id)
		results = append(results, result)
		return nil
	})
	for resId, funcs := range resources {
		wp.AddTask(func() (any, error) {
			result := Result{Id: resId}
			if f := funcs.R; f != nil {
				result.Read = resReachSDK(graph, funcs.R, sdkFunctions)
			}
			if !resId.IsDataSource {
				if f := funcs.C; f != nil {
					result.Create = resReachSDK(graph, funcs.C, sdkFunctions)
					// Union the read functions as create will always call the read at the end.
					// This is not necessary for untyped sdk as the read is called explicitly,
					// while it is necessary for the typed sdk, as the read is implicitly called via the framework.
					result.Create.Union(result.Read)
				}
				if f := funcs.U; f != nil {
					// Union the read functions as update will always call the read at the end.
					// This is not necessary for untyped sdk as the read is called explicitly,
					// while it is necessary for the typed sdk, as the read is implicitly called via the framework.
					result.Update = resReachSDK(graph, funcs.U, sdkFunctions)
					result.Update.Union(result.Read)
				}
				if f := funcs.D; f != nil {
					result.Delete = resReachSDK(graph, funcs.D, sdkFunctions)
				}
			}
			return result, nil
		})

	}

	if err := wp.Done(); err != nil {
		log.Fatal(err)
	}

	sort.Sort(results)

	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("marshal the result: %v", err)
	}

	fmt.Println(string(b))
}
