package main

// The program takes a PDF file as argument and recursively dump the content.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/pdutil"
	"github.com/raff/pdfreader/util"
)

var (
	maxlevel = 0
)

func PrintRef(w io.Writer, pd *pdfread.PdfReaderT, ref string, fmtref string) {
	if !strings.HasSuffix(ref, " R") {
		ref = fmt.Sprintf("%v 0 R", ref)
	}

	obj := pd.Obj([]byte(ref))

	// print resource info
	pdutil.Printobj(w, pd, obj, "", ref, maxlevel, fmtref)
}

func main() {
	flag.BoolVar(&util.Debug, "debug", false, "enable debug logging")
	flag.BoolVar(&pdutil.Debugobj, "dump", false, "dump object content")
	flag.IntVar(&maxlevel, "levels", 5, "maximum number of levels")
	displayref := flag.String("r", "", "display resource by reference")

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

		if *displayref != "" {
			PrintRef(os.Stdout, pd, *displayref, "")
			fmt.Println()
			break
		}

		pdutil.Printdic(os.Stdout, pd, pd.Trailer, "", "/Trailer", maxlevel, "")
		fmt.Println()
	}
}
