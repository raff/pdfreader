package main

// The program takes a PDF file as argument and recursively dump the content.

import (
	"flag"
	"fmt"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/pdutil"
	"github.com/raff/pdfreader/util"
)

var (
	debugobj = false
	maxlevel = 0
)

func main() {
	flag.BoolVar(&util.Debug, "debug", false, "enable debug logging")
	flag.BoolVar(&debugobj, "dump", false, "dump object content")
	flag.IntVar(&maxlevel, "levels", 5, "maximum number of levels")

	flag.Parse()

	for _, f := range flag.Args() {
		fmt.Println("----", f, "--------------------")

		pd := pdfread.Load(f)
		if pd == nil {
			fmt.Println("can't open input file")
			fmt.Println()
			continue
		}

		fmt.Println("Trailer {")
		for k, v := range pd.Trailer {
			fmt.Printf("  %s %q\n", k, v)
		}
		fmt.Println("}")
		fmt.Println()

		root := pd.Trailer["/Root"]

		pdutil.Printobj(pd, root, "", "/Root", maxlevel)
		fmt.Println()
	}
}
