package pdutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/util"
)

var (
	Debugobj = false
)

func IsRef(o []byte) bool {
	return bytes.HasSuffix(o, []byte(" R"))
}

func Printobj(w io.Writer, pd *pdfread.PdfReaderT, o []byte, indent, prefix string, maxlevel int, fmtref string) {
	if IsRef(o) {
		ref := fmt.Sprintf("<<%s>>", o)

		//if Debugobj {
		//        fmt.Fprintf(w, "%% %s\n", o)
		//}

		o = pd.Obj(o)

		if prefix == "" {
			prefix = ref
		} else {
			prefix += " " + ref
		}
	}

	if len(o) == 0 {
		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "<<empty>>")
		return
	}

	if Debugobj {
		fmt.Fprintf(w, "%% %s\n", o)
	}

	maxlevel -= 1

	switch o[0] {
	case '[': // array
		a := pdfread.Array(o)

		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "[")
		indent += "  "

		if maxlevel < 0 {
			fmt.Fprintf(w, "%s<<more>>\n", indent)
		} else {
			for i, v := range a {
				if fmtref != "" && IsRef(v) {
					fmt.Fprintf(w, "%s%d: ", indent, i)
					fmt.Fprintf(w, fmtref, v)
					fmt.Fprintln(w)
				} else {
					Printobj(w, pd, v, indent, fmt.Sprintf("%d:", i), maxlevel, fmtref)
				}
			}
		}

		indent = indent[2:]
		fmt.Fprintf(w, "%s]\n", indent)

	case '<': // dictionary
		d := pdfread.Dictionary(o)

		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "{")
		indent += "  "

		if maxlevel < 0 {
			fmt.Fprintf(w, "%s<<more>>\n", indent)
		} else {
			for k, v := range d {
				if fmtref != "" && IsRef(v) {
					fmt.Fprintf(w, "%s%s ", indent, k)
					fmt.Fprintf(w, fmtref, v)
					fmt.Fprintln(w)
				} else {
					if k == "/Parent" { // backreference - don't follow
						vv := string(v)
						if fmtref != "" {
							vv = fmt.Sprintf(fmtref, v)
						}

						fmt.Fprintf(w, "%s%s <<%s>>\n", indent, k, vv)
					} else {
						Printobj(w, pd, v, indent, k, maxlevel, fmtref)
					}
				}
			}
		}

		indent = indent[2:]
		fmt.Fprintf(w, "%s}\n", indent)

	case '/': // symbol
		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, util.Unescape(o))

	case '(': // string
		fmt.Fprintf(w, "%s%s %q\n", indent, prefix, util.String(o))

	default:
		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, string(o))
	}
}

func Printdic(w io.Writer, pd *pdfread.PdfReaderT, d pdfread.DictionaryT, indent, prefix string, maxlevel int, fmtref string) {
	fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "{")
	indent += "  "

	for k, v := range d {
		if k == "/Parent" { // backreference - don't follow
			vv := string(v)
			if fmtref != "" {
				vv = fmt.Sprintf(fmtref, v)
			}

			fmt.Fprintf(w, "%s%s <<%s>>\n", indent, k, vv)
		} else {
			Printobj(w, pd, v, indent, k, maxlevel, fmtref)
		}
	}

	indent = indent[2:]
	fmt.Fprintf(w, "%s}\n", indent)
}
