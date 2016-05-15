package main

import (
	"fmt"
	"github.com/raff/pdfreader/pdfread"
	"os"
)

// Example program for pdfread.go

// The program takes a PDF file as argument and writes the MediaBoxes and
// defined fonts of the pages.

func main() {
	pd := pdfread.Load(os.Args[1])
	if pd != nil {
		fmt.Println("PageMode", pd.PageMode)
		fmt.Println()

		outlines := pd.Outlines()

		if len(outlines) > 0 {
			fmt.Println("Outlines:")
			for _, p := range outlines {
				fmt.Println("Page", p.Page+1, "-", p.Title)
			}

			fmt.Println()
		}

		fmt.Println("Pages:")
		pg := pd.Pages()

		for k := range pg {
			fmt.Printf("Page %d - MediaBox: %s\n",
				k+1, pd.Att("/MediaBox", pg[k]))
			fonts := pd.PageFonts(pg[k])
			for l := range fonts {
				fontname := pd.Dic(fonts[l])["/BaseFont"]
				fmt.Printf("  %s = \"%s\"\n",
					l, fontname[1:])
			}
		}
	}
}
