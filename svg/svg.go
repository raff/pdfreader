// Copyright (c) 2009 Helmar Wodtke. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// The MIT License is an OSI approved license and can
// be found at
//   http://www.opensource.org/licenses/mit-license.php

// Library to convert PDF pages to SVG.
package svg

import (
	"fmt"
	"github.com/raff/pdfreader/fancy"
	"github.com/raff/pdfreader/graf"
	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/strm"
	"github.com/raff/pdfreader/svgdraw"
	"github.com/raff/pdfreader/svgtext"
	"github.com/raff/pdfreader/util"
	"os"
)

func complain(err string) {
	fmt.Printf("%s", err)
	os.Exit(1)
}

func Page(pd *pdfread.PdfReaderT, page int, xmlDecl bool) []byte {
	pg := pd.Pages()
	if page >= len(pg) {
		complain("Page does not exist!\n")
	}
	mbox := util.StringArray(pd.Arr(pd.Att("/MediaBox", pg[page])))

	rdict := pd.Dic(pd.Att("/Resources", pg[page]))
	resources := graf.ResourcesT{ColorSpaces: map[string]graf.ColorSpaceT{}}

	if cs := pd.Dic(rdict["/ColorSpace"]); cs != nil {
		for name, ref := range cs {
			values := pd.Arr(ref)

			ctype := string(values[0])
			n := 0

			switch ctype {
			case "/ICCBased":
				dic, _ := pd.Stream(values[1])
				n = pd.Num(dic["/N"])

			case "/DeviceCMYK":
				n = 4

			case "/CalRGB", "/Lab", "/DeviceRGB":
				n = 3

			case "/CalGray", "/DeviceGray": // maybe /Separation and /Indexed
				n = 1
			}

			resources.ColorSpaces[name] = graf.ColorSpaceT{Type: ctype, N: n}
			util.Logf("ColorSpace %v %v", name, resources.ColorSpaces[name])
		}
	}

	drw := svgdraw.NewTestSvg(resources)
	svgtext.New(pd, drw).Page = page
	w := strm.Mul(strm.Sub(mbox[2], mbox[0]), "1.25")
	h := strm.Mul(strm.Sub(mbox[3], mbox[1]), "1.25")
	decl := "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n"
	if !xmlDecl {
		decl = ""
	}

	drw.Write.Out("%s"+
		"<svg\n"+
		"   xmlns:svg=\"http://www.w3.org/2000/svg\"\n"+
		"   xmlns=\"http://www.w3.org/2000/svg\"\n"+
		"   version=\"1.0\"\n"+
		"   width=\"%s\"\n"+
		"   height=\"%s\">\n"+
		"<g transform=\"matrix(1.25,0,0,-1.25,%s,%s)\">\n",
		decl,
		w, h,
		strm.Mul(mbox[0], "-1.25"),
		strm.Mul(mbox[3], "1.25"))
	cont := pd.ForcedArray(pd.Dic(pg[page])["/Contents"])
	_, ps := pd.DecodedStream(cont[0])
	drw.Interpret(fancy.SliceReader(ps))
	drw.Draw.CloseDrawing()
	drw.Write.Out("</g>\n</svg>\n")
	return drw.Write.Content
}
