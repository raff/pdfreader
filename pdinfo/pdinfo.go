package main

// The program takes a PDF file as argument and recursively dump the content.

import (
	"flag"
	"fmt"
	"os"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/pdutil"
	"github.com/raff/pdfreader/util"
)

var (
	maxlevel = 0
)

func main() {
	flag.BoolVar(&util.Debug, "debug", false, "enable debug logging")
	flag.BoolVar(&pdutil.Debugobj, "dump", false, "dump object content")
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

		fmt.Println(pd.Version)
		fmt.Println()

		pdutil.Printdic(os.Stdout, pd, pd.Trailer, "", "/Trailer", maxlevel, "")
		fmt.Println()

		/*
			fmt.Println("Trailer {")
			for k, v := range pd.Trailer {
				fmt.Printf("  %s %q\n", k, v)
			}
			fmt.Println("}")
			fmt.Println()

			root := pd.Trailer["/Root"]

			pdutil.Printobj(os.Stdout, pd, root, "", "/Root", maxlevel)
			fmt.Println()
		*/
	}
}
