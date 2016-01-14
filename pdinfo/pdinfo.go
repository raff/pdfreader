package main

// The program takes a PDF file as argument and recursively dump the content.

import (
	"flag"
	"fmt"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/util"
)

var (
	debugobj = false
	maxlevel = 0
)

func printobj(pd *pdfread.PdfReaderT, o []byte, indent, prefix string, maxlevel int) {
	maxlevel -= 1
	l := len(o)

	if l > 2 {
		if o[l-2] == ' ' && o[l-1] == 'R' { // reference
			ref := fmt.Sprintf("<<%s>>", o)
			o = pd.Obj(o)

			if prefix == "" {
				prefix = ref
			} else {
				prefix += " " + ref
			}
		}
	}

	if l == 0 {
		fmt.Printf("%s%s %s\n", indent, prefix, "<<empty>>")
		return
	}

	/*
	   if l < 2 {
	       fmt.Printf("%s%s %s\n", indent, prefix, "<<invalid>>")
	       return
	   }
	*/

	if debugobj {
		fmt.Println(string(o))
	}

	switch o[0] {
	case '[': // array
		a := pdfread.Array(o)

		fmt.Printf("%s%s %s\n", indent, prefix, "[")
		indent += "  "

		if maxlevel < 0 {
			fmt.Printf("%s<<more>>\n", indent)
		} else {
			for i, v := range a {

				printobj(pd, v, indent, fmt.Sprintf("%d:", i), maxlevel)
			}
		}

		indent = indent[2:]
		fmt.Printf("%s]\n", indent)

	case '<': // dictionary
		d := pdfread.Dictionary(o)

		fmt.Printf("%s%s %s\n", indent, prefix, "{")
		indent += "  "

		if maxlevel < 0 {
			fmt.Printf("%s<<more>>\n", indent)
		} else {
			for k, v := range d {
				if k == "/Parent" { // backreference - don't follow
					fmt.Printf("%s%s <<%s>>\n", indent, k, string(v))
				} else {
					printobj(pd, v, indent, k, maxlevel)
				}
			}
		}

		indent = indent[2:]
		fmt.Printf("%s}\n", indent)

	case '/': // symbol
		fmt.Printf("%s%s %s\n", indent, prefix, util.Unescape(o))

	default:
		fmt.Printf("%s%s %s\n", indent, prefix, string(o))
	}
}

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

		printobj(pd, root, "", "/Root", maxlevel)
		fmt.Println()
	}
}
