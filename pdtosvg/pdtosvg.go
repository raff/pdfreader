// Copyright (c) 2009 Helmar Wodtke. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// The MIT License is an OSI approved license and can
// be found at
//   http://www.opensource.org/licenses/mit-license.php

// Convert PDF-pages to SVG.
package main

import (
        "flag"
	"fmt"
	"github.com/raff/pdfreader/pdfread"
	//"github.com/raff/pdfreader/strm"
	"github.com/raff/pdfreader/svg"
	"github.com/raff/pdfreader/util"
	"os"
)

// The program takes a PDF file and converts a page to SVG.

func complain(err string) {
	fmt.Printf("%susage: pdtosvg [--html] [--page=n] foo.pdf >foo.svg\n", err)
	os.Exit(1)
}

func main() {
        asHtml := flag.Bool("html", false, "output as html (true) or xml (false)")
        debug := flag.Bool("debug", false, "debug mode")
        page := flag.Int("page", 0, "page number")

        flag.Parse()

        if *page < 0 {
                complain("Bad page!\n\n")
        }

        if flag.NArg() != 1 {
                complain("")
        }

	pd := pdfread.Load(flag.Arg(0))
	if pd == nil {
		complain("Could not load pdf file!\n\n")
	}

        util.Debug = *debug

        if *asHtml {
	    fmt.Printf("<!DOCTYPE html><html><body>\n%s</body></html>\n", svg.Page(pd, *page, false))
        } else {
	    fmt.Printf("%s", svg.Page(pd, *page, true))
        }
}
