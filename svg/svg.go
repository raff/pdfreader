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
	resources := graf.ResourcesT{ColorSpaces: map[string]graf.ColorSpaceT{},
                                     GraphicsStates: map[string]graf.DrawerConfigT{}}

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

        if gs := pd./Dic(rdict["/ExtGState"); gs != nil {
                gstate := graf.DraweConfigT{}

                for name, val := range gs {
                    switch name {
                    case "LW": // number: line width
                        gstate.LineWidth = string(val)
                    case "LC": // int:    line cap style
                        gstate.LineCap = string(val)
                    case "LJ": // int:    line join style
                        gstate.LineJoin = string(val)
                    case "ML": // number: miter limit
                        gstate.MiterLimit = string(val)
                    case "D":  // array:  line dash pattern
                    case "RI": // name:   rendering intent
                    case "OP": // boolean: overprint - all painting operations
                        bval := string(val) == "true"
                        gstate.Overprint = bval
                        gstate.OverprintStroke = bval
                    case "op": // boolean: overprint - all operations but strokes
                        bval := string(val) == "true"
                        gstate.Overprint = bval
                    case "OPM":  // int: overprint mode
                    case "Font": // array: [font size]
                    case "BG":   // function: black generation function
                    case "BG2":  // function: black generation function or "Default"
                    case "UCR":  // function: undercolor removal function
                    case "UCR2":  // function: undercolor removal function or "Default"
                    case "TR":   // function, array[4] or "Identity": transfer function
                    case "HR":   // dictionary, stream or "Default": halftone dictionary or stream
                    case "FL":   // number: flatness tolerance
                    case "SM":   // number: smoothness tolerance
                    case "SA":   // boolean: automatic stroke adjustment
                    case "BM":   // name or array: blend mode
                    case "SMask": // dictionary or name: current soft mask
                    case "CA": // number: current stroking alpha constant
                    case "ca": // number: current alpha constant for non-stroking operations
                    case "AIS": // boolean: alpha source flag (alpha is shape)
                    case "TK": // boolean: text knockout flag
                    }


                    resources.GraphicsStates[name] = gstate
                    util.Logf("GraphicsState %v %v", name, gstate)
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
