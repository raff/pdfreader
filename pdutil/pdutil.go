package pdutil

import (
	"fmt"
	"io"

	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/util"
)

var (
	Debugobj = false
)

func Printobj(w io.Writer, pd *pdfread.PdfReaderT, o []byte, indent, prefix string, maxlevel int) {
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
		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "<<empty>>")
		return
	}

	if Debugobj {
		fmt.Fprintf(w, "%% %s", o)
	}

	switch o[0] {
	case '[': // array
		a := pdfread.Array(o)

		fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "[")
		indent += "  "

		if maxlevel < 0 {
			fmt.Fprintf(w, "%s<<more>>\n", indent)
		} else {
			for i, v := range a {

				Printobj(w, pd, v, indent, fmt.Sprintf("%d:", i), maxlevel)
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
				if k == "/Parent" { // backreference - don't follow
					fmt.Fprintf(w, "%s%s <<%s>>\n", indent, k, string(v))
				} else {
					Printobj(w, pd, v, indent, k, maxlevel)
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

func Printdic(w io.Writer, pd *pdfread.PdfReaderT, d pdfread.DictionaryT, indent, prefix string, maxlevel int) {
	fmt.Fprintf(w, "%s%s %s\n", indent, prefix, "{")
	indent += "  "

	for k, v := range d {
		if k == "/Parent" { // backreference - don't follow
			fmt.Fprintf(w, "%s%s <<%s>>\n", indent, k, string(v))
		} else {
			Printobj(w, pd, v, indent, k, maxlevel)
		}
	}

	indent = indent[2:]
	fmt.Fprintf(w, "%s}\n", indent)
}
