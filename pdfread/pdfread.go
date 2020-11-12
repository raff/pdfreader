// Copyright (c) 2009 Helmar Wodtke. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// The MIT License is an OSI approved license and can
// be found at
//   http://www.opensource.org/licenses/mit-license.php

// Access to PDF files.
package pdfread

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/hex"
	"regexp"

	"github.com/raff/pdfreader/fancy"
	"github.com/raff/pdfreader/lzw"
	"github.com/raff/pdfreader/ps"
	"github.com/raff/pdfreader/util"
)

// limits

const (
	MAX_PDF_UPDATES   = 1024
	MAX_PDF_ARRAYSIZE = 1024
)

// types

type DictionaryT map[string][]byte

type PdfReaderT struct {
	File      string            // name of the file
	Size      int64             // file size
	Version   string            // PDF version
	rdr       fancy.Reader      // reader for the contents
	Startxref int               // starting of xref table
	Xref      map[int]int       // "pointers" of the xref table
	Trailer   DictionaryT       // trailer dictionary of the file
	PageMode  string            // /Root/PageMode
	rcache    map[string][]byte // resolver cache
	rncache   map[string]int    // resolver cache (positions in file)
	dicache   map[string]DictionaryT
	pages     [][]byte // pages cache
}

var _Bytes = []byte{}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func end(a []byte, n int) int { return max(0, len(a)-n) }

func num(n []byte) (r int) {
	mul := 1

	for i := 0; i < len(n); i++ {
		if i == 0 && n[i] == '-' {
			mul = -1
			continue
		}
		if n[i] >= '0' && n[i] <= '9' {
			r = r*10 + int(n[i]-'0')
		} else {
			break
		}
	}
	return r * mul
}

func numdef(n []byte, defn int) int {
	if n == nil {
		return defn
	} else {
		return num(n)
	}
}

func bnum(n []byte) (ret int) {
	for _, b := range n {
		ret = (ret << 8) + int(b)
	}

	return
}

func refToken(f fancy.Reader) ([]byte, int64) {
	tok, p := ps.Token(f)
	if len(tok) > 0 && tok[0] >= '0' && tok[0] <= '9' {
		ps.Token(f)
		r, q := ps.Token(f)
		if string(r) == "R" {
			f.Seek(p, 0)
			tok = f.Slice(int(1 + q - p))
		} else {
			f.Seek(p+int64(len(tok)), 0)
		}
	}
	return tok, p
}

func tuple(f fancy.Reader, count int) [][]byte {
	r := make([][]byte, count)
	for i := 0; i < count; i++ {
		r[i], _ = ps.Token(f)
	}
	return r
}

var startxref = regexp.MustCompile(
	"startxref[\t ]*(\r?\n|\r)[\t ]*([0-9]+)[\t ]*(\r?\n|\r)[\t ]*%%EOF")

// xrefStart() queries the start of the xref-table in a PDF file.
func xrefStart(f fancy.Reader) int {
	s := int(f.Size())
	pdf := make([]byte, min(s, 1024))
	f.ReadAt(pdf, int64(max(0, s-1024)))
	ps := startxref.FindAll(pdf, -1)
	if ps == nil {
		return -1
	}
	return num(startxref.FindSubmatch(ps[len(ps)-1])[2])
}

// xrefSkip() queries the start of the trailer for a (partial) xref-table.
func xrefSkip(f fancy.Reader, xref int) int {
	f.Seek(int64(xref), 0)
	t, p := ps.Token(f)
	if string(t) != "xref" {
		return -1
	}
	for {
		t, p = ps.Token(f)
		if t[0] < '0' || t[0] > '9' {
			f.Seek(p, 0)
			break
		}
		t, _ = ps.Token(f)
		ps.SkipLE(f)
		f.Seek(int64(num(t)*20), 1)
	}
	r, _ := f.Seek(0, 1)
	return int(r)
}

// Dictionary() makes a map/hash from PDF dictionary data.
func Dictionary(s []byte) DictionaryT {
	if len(s) < 4 {
		return nil
	}
	e := len(s) - 1
	if s[0] != s[1] || s[0] != '<' || s[e] != s[e-1] || s[e] != '>' {
		return nil
	}
	r := make(DictionaryT)
	rdr := fancy.SliceReader(s[2 : e-1])
	for {
		t, _ := ps.Token(rdr)
		if len(t) == 0 {
			break
		}
		if t[0] != '/' {
			return nil
		}
		k := string(t)
		t, _ = refToken(rdr)
		r[k] = t
	}
	return r
}

// Array() extracts an array from PDF data.
func Array(s []byte) [][]byte {
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil
	}
	rdr := fancy.SliceReader(s[1 : len(s)-1])
	r := make([][]byte, 0, MAX_PDF_ARRAYSIZE)
	for {
		v, _ := refToken(rdr)
		if len(v) == 0 {
			break
		}
		r = append(r, v)
	}
	if len(r) == 0 {
		return nil
	}
	return r
}

func ForcedArray(s []byte) [][]byte {
	if s[0] != '[' {
		s = append([]byte{'['}, s...)
		s = append(s, ']')
	}
	return Array(s)
}

// xrefReadTable() reads the xref table(s) of a PDF file. This is not recursive
// in favour of not to have to keep track of already used starting points
// for xrefs.
func xrefReadTable(f fancy.Reader, p int) (map[int]int, DictionaryT) {
	var trailer DictionaryT
	var back [MAX_PDF_UPDATES]int
	b := 0
	s := _Bytes
	for ok := true; ok; {
		back[b] = p
		b++
		p = xrefSkip(f, p)
		f.Seek(int64(p), 0)
		s, _ = ps.Token(f)
		if string(s) != "trailer" {
			util.Log("no trailer / xref table")
			return nil, nil
		}
		s, _ = ps.Token(f)
		if trailer == nil {
			trailer = Dictionary(s)
		}
		s, ok = Dictionary(s)["/Prev"]
		p = num(s)
	}
	r := make(map[int]int)
	for b != 0 {
		b--
		f.Seek(int64(back[b]), 0)
		ps.Token(f) // skip "xref"
		for {
			m := tuple(f, 2)
			if string(m[0]) == "trailer" {
				break
			}
			ps.SkipLE(f)
			o := num(m[0])
			dat := f.Slice(num(m[1]) * 20)
			for i := 0; i < len(dat); i += 20 {
				if dat[i+17] != 'n' {
					delete(r, o)
				} else {
					r[o] = num(dat[i : i+10])
				}
				o++
			}
		}
	}
	return r, trailer
}

// xrefReadStream() reads the xref stream(s) of a PDF file. This is not recursive
// in favour of not to have to keep track of already used starting points
// for xrefs.
func xrefReadStream(f fancy.Reader, p int) (xr map[int]int, r [][2]int, trailer DictionaryT) {
	s := _Bytes

	xr = map[int]int{}
	r = [][2]int{}

	for ok := true; ok; {
		f.Seek(int64(p), 0)
		ps.Token(f) // skip "xref"

		//for {
		m := tuple(f, 2)
		if string(m[1]) != "obj" {
			util.Logf("unexpected %q\n", m)
			return nil, nil, nil
		}

		s, _ = ps.Token(f)
		dic := Dictionary(s)

		if trailer == nil {
			trailer = dic
		}

		s, ok = dic["/Prev"]
		p = num(s)

		s, _ = ps.Token(f)
		if string(s) != "stream" {
			util.Log("not a stream", s)
			return nil, nil, nil
		}
		ps.SkipLE(f)

		for k, v := range dic {
			util.Logf("%s %s", k, v)
		}

		size := num(dic["/Size"])
		index := []int{0, size}

		if _, ok := dic["/Index"]; ok {
			a := Array(dic["/Index"])
			index[0] = num(a[0])
			index[1] = num(a[1])

			// can len(index) be != 2 ?
		}

		l := num(dic["/Length"])
		xref := f.Slice(l)

		w := Array(dic["/W"])
		if len(w) != 3 {
			util.Log("unexpected /W", w)
		}

		fl1 := num(w[0])
		fl2 := num(w[1])
		fl3 := num(w[2])

		width := fl1 + fl2 + fl3

		xref = decodeStream(dic, xref)

		s, _ = ps.Token(f) // endstream
		s, _ = ps.Token(f) // endobj

		pos := index[0]

		for i := 0; i < len(xref); i += width {
			ent := xref[i : i+width]
			f1 := bnum(ent[0:fl1])
			f2 := bnum(ent[fl1 : fl1+fl2])
			f3 := bnum(ent[fl1+fl2:])

			switch f1 {
			case 0:
				// free object
				util.Log("free", f2, f3)
				// delete(r, f2)

			case 1:
				// regular object
				util.Log("ref", pos, f3, f2)
				xr[pos] = f2

			case 2:
				// compressed object
				util.Log("cref", pos, f2, f3)
				r = append(r, [2]int{f2, f3})
			}

			pos += 1
		}
		//}
	}

	return xr, r, trailer
}

// object() extracts the top informations of a PDF "object". For streams
// this would be the dictionary as bytes.  It also returns the position in
// binary data where one has to continue to read for this "object".
func (pd *PdfReaderT) object(o int) (int, []byte) {
	p, ok := pd.Xref[o]
	if !ok {
		return -1, _Bytes
	}
	pd.rdr.Seek(int64(p), 0)
	m := tuple(pd.rdr, 3)
	if num(m[0]) != o {
		return -1, _Bytes
	}
	r, np := refToken(pd.rdr)
	return int(np) + len(r), r
}

// pd.Resolve() resolves a reference in the PDF file. You'll probably need
// this method for reading streams only.
func (pd *PdfReaderT) Resolve(s []byte) (int, []byte) {
	if len(s) < 5 || s[len(s)-1] != 'R' {
		return -1, s
	}
	done := make(map[int]int)
	var resolve func(s []byte) (int, []byte)
	resolve = func(s []byte) (int, []byte) {
		n := -1
		if len(s) >= 5 && s[0] >= '0' && s[0] <= '9' && s[len(s)-1] == 'R' {
			z, ok := pd.rcache[string(s)]
			if ok {
				return pd.rncache[string(s)], z
			}
			orig := s
			o := num(s)
			if _, ok = done[o]; ok {
				return -1, _Bytes
			}
			done[o] = 1
			n, s = pd.object(o)
			if len(s) >= 5 && s[0] >= '0' && s[0] <= '9' && s[len(s)-1] == 'R' {
				n, s = resolve(s)
			}
			pd.rcache[string(orig)] = s
			pd.rncache[string(orig)] = n
		}
		return n, s
	}
	return resolve(s)
}

// pd.Obj() is the universal method to access contents of PDF objects or
// data tokens in i.e.  dictionaries.  For reading streams you'll have to
// utilize pd.Resolve().
func (pd *PdfReaderT) Obj(reference []byte) []byte {
	_, r := pd.Resolve(reference)
	return r
}

// pd.Num() queries integer data from a reference.
func (pd *PdfReaderT) Num(reference []byte) int {
	return num(pd.Obj(reference))
}

// pd.Dic() queries dictionary data from a reference.
func (pd *PdfReaderT) Dic(reference []byte) DictionaryT {
	d, ok := pd.dicache[string(reference)]
	if !ok {
		d = Dictionary(pd.Obj(reference))
		pd.dicache[string(reference)] = d
	}
	return d
}

// pd.Arr() queries array data from a reference.
func (pd *PdfReaderT) Arr(reference []byte) [][]byte {
	return Array(pd.Obj(reference))
}

// pd.ForcedArray() queries array data. If reference does not refer to an
// array, reference is taken as element of the returned array.
func (pd *PdfReaderT) ForcedArray(reference []byte) [][]byte {
	nr := pd.Obj(reference)
	if nr[0] != '[' {
		return [][]byte{reference}
	}
	return Array(nr)
}

// pd.Pages() returns an array with references to the pages of the PDF.
func (pd *PdfReaderT) Pages() [][]byte {
	if pd.pages != nil {
		return pd.pages
	}
	pages := pd.Dic(pd.Dic(pd.Trailer["/Root"])["/Pages"])
	pd.pages = make([][]byte, pd.Num(pages["/Count"]))
	cp := 0
	done := make(map[string]int)
	var q func(p [][]byte)
	q = func(p [][]byte) {
		for k := range p {
			if _, wrong := done[string(p[k])]; !wrong {
				done[string(p[k])] = 1
				if kids, ok := pd.Dic(p[k])["/Kids"]; ok {
					q(pd.Arr(kids))
				} else {
					pd.pages[cp] = p[k]
					cp++
				}
			} else {
				panic("Bad Page-Tree!")
			}
		}
	}
	q(pd.Arr(pages["/Kids"]))
	return pd.pages
}

type Outline struct {
	Title string
	Page  int
	Ref   []byte
}

func (pd *PdfReaderT) pageNum(p []byte) int {
	for i, pg := range pd.Pages() {
		if bytes.Equal(p, pg) {
			return i
		}
	}

	return -1
}

func (pd *PdfReaderT) Outlines() []Outline {
	if pd.PageMode != "/UseOutlines" {
		return nil
	}

	d := pd.Dic(pd.Dic(pd.Trailer["/Root"])["/Outlines"])
	outlines := make([]Outline, pd.Num(d["/Count"]))

	p := pd.Dic(d["/First"])
	for i := 0; p != nil; i++ {
		outlines[i].Ref = pd.Arr(p["/Dest"])[0]
		outlines[i].Page = pd.pageNum(outlines[i].Ref)
		outlines[i].Title = string(ps.String(p["/Title"]))
		p = pd.Dic(p["/Next"])
	}

	return outlines
}

// pd.Attribute() tries to get an attribute definition from a page
// reference.  Note that the attribute definition is not resolved - so it's
// possible to get back a reference here.
func (pd *PdfReaderT) Attribute(a string, src []byte) []byte {
	d := pd.Dic(src)
	done := make(map[string]int)
	r, ok := d[a]
	for !ok {
		r, ok = d["/Parent"]
		if _, wrong := done[string(r)]; wrong || !ok {
			return _Bytes
		}
		done[string(r)] = 1
		d = pd.Dic(r)
		r, ok = d[a]
	}
	return r
}

// pd.Att() tries to get an attribute from a page reference.  The
// attribute will be resolved.
func (pd *PdfReaderT) Att(a string, src []byte) []byte {
	return pd.Obj(pd.Attribute(a, src))
}

// pd.Stream() returns contents of a stream.
func (pd *PdfReaderT) Stream(reference []byte) (DictionaryT, []byte) {
	q, d := pd.Resolve(reference)
	dic := pd.Dic(d)
	l := pd.Num(dic["/Length"])
	pd.rdr.Seek(int64(q), 0)
	t, _ := ps.Token(pd.rdr)
	if string(t) != "stream" {
		util.Log("Stream", string(reference), "not a stream", string(t))
		return nil, []byte{}
	}
	ps.SkipLE(pd.rdr)
	return dic, pd.rdr.Slice(l)
}

// pd.DecodedStream() returns decoded contents of a stream.
func (pd *PdfReaderT) DecodedStream(reference []byte) (DictionaryT, []byte) {
	dic, data := pd.Stream(reference)
	return dic, decodeStream(dic, data)
}

func decodeStream(dic DictionaryT, data []byte) []byte {
	if f, ok := dic["/Filter"]; ok {
		filter := ForcedArray(f)
		var decos [][]byte
		if d, ok := dic["/DecodeParms"]; ok {
			decos = ForcedArray(d)
		} else {
			decos = make([][]byte, len(filter))
		}

		for ff := range filter {
			// XXX: if there are multiple filters but only one DecodeParams,
			//      it should be used for all filters

			deco := Dictionary(decos[ff])
			switch string(filter[ff]) {
			case "/FlateDecode":
				data = fancy.ReadAndClose(zlib.NewReader(fancy.SliceReader(data)))
			case "/LZWDecode":
				early := true
				if deco != nil {
					if s, ok := deco["/EarlyChange"]; ok {
						early = num(s) == 1
					}
				}
				data = lzw.Decode(data, early)
			case "/ASCII85Decode":
				ds := data
				for len(ds) > 1 && ds[len(ds)-1] < 33 {
					ds = ds[0 : len(ds)-1]
				}
				if len(ds) >= 2 && ds[len(ds)-1] == '>' && ds[len(ds)-2] == '~' {
					ds = ds[0 : len(ds)-2]
				}
				data = fancy.ReadAll(ascii85.NewDecoder(fancy.SliceReader(ds)))
			case "/ASCIIHexDecode":
				data, _ = hex.DecodeString(string(data))
			default:
				util.Log("Unsupported filter", string(filter[ff]))
				data = []byte{}
			}

			if s, ok := deco["/Predictor"]; ok {
				pred := num(s)

				switch {
				case pred == 1:
					// no predictor

				case pred > 10:
					colors := numdef(deco["/Colors"], 1)
					columns := numdef(deco["/Columns"], 1)
					bitspercomponent := numdef(deco["/BitsPerComponent"], 8)

					util.Log("applying predictor", pred, colors, columns, bitspercomponent)
					data = util.ApplyPNGPredictor(pred, colors, columns, bitspercomponent, data)

				default:
					util.Log("Unsupported predictor", pred)
					return nil
				}
			}
		}
	}
	return data
}

// pd.PageFonts() returns references to the fonts defined for a page.
func (pd *PdfReaderT) PageFonts(page []byte) DictionaryT {
	fonts, _ := pd.Dic(pd.Attribute("/Resources", page))["/Font"]
	if fonts == nil {
		return nil
	}
	return pd.Dic(fonts)
}

// pd.Close() closes the underlying file and releases all resources
func (pd *PdfReaderT) Close() {
	pd.rdr.Close()
	pd.File = ""
	pd.Version = ""
	pd.rdr = nil
	pd.Startxref = -1
	pd.Xref = nil
	pd.Trailer = nil
	pd.PageMode = ""
	pd.rcache = nil
	pd.rncache = nil
	pd.dicache = nil
	pd.pages = nil
}

// Load() loads a PDF file of a given name.
func Load(fn string) *PdfReaderT {
	return load(fn, fancy.FileReader(fn))
}

// LoadBytes() loads a PDF file from a byte buffer.
func LoadBytes(b []byte) *PdfReaderT {
	return load("<buffer>", fancy.BytesReader(b))
}

func load(fn string, fr fancy.Reader) *PdfReaderT {
	var rr [][2]int // list of entries to resolve

	r := new(PdfReaderT)
	r.File = fn
	r.rdr = fr
	if r.rdr == nil {
		util.Log(fn, "FileReader error")
		return nil
	}

	r.Size = r.rdr.Size()

	v := make([]byte, 16)
	r.rdr.ReadAt(v, 0)

	if v[0] != '%' || v[1] != 'P' || v[2] != 'D' || v[3] != 'F' {
		util.Log(string(v), "not a PDF")
		r.rdr.Close()
		return nil
	}

	x := bytes.IndexByte(v, '\r')
	if x > 0 {
		v = v[:x]
	}
	r.Version = string(v)

	if r.Startxref = xrefStart(r.rdr); r.Startxref == -1 {
		util.Log(fn, "xrefStart error")
		r.rdr.Close()
		return nil
	}

	if r.Xref, r.Trailer = xrefReadTable(r.rdr, r.Startxref); r.Xref == nil {
		r.Xref, rr, r.Trailer = xrefReadStream(r.rdr, r.Startxref)
	}

	if r.Xref == nil {
		util.Log(fn, "xrefRead error")
		r.rdr.Close()
		return nil
	}

	if r.Trailer == nil {
		r.rdr.Seek(int64(xrefSkip(r.rdr, r.Startxref)), 0)

		s, _ := ps.Token(r.rdr)
		if string(s) != "trailer" {
			util.Log(fn, "no trailer")
			r.rdr.Close()
			return nil
		}
		s, _ = ps.Token(r.rdr)
		if r.Trailer = Dictionary(s); r.Trailer == nil {
			util.Log(fn, "no trailer dictionary")
			r.rdr.Close()
			return nil
		}
	}

	r.rcache = make(map[string][]byte)
	r.rncache = make(map[string]int)
	r.dicache = make(map[string]DictionaryT)

	if rr != nil {
		curr := -1

		var dic DictionaryT
		var s []byte

		for _, v := range rr {
			o, i := v[0], v[1]

			if o != curr {
				curr = o

				dic, s = r.DecodedStream(util.MakeRef(curr))

				first := num(dic["/First"])
				n := num(dic["/N"])

				rdr := fancy.SliceReader(s)

				p := tuple(rdr, n*2)

				util.Log("Object-Stream", curr)
				for i := 0; i < len(p); i += 2 {
					oo := num(p[i+0])
					offs := num(p[i+1])

					util.Log(oo, first+offs)

					rdr.Seek(int64(first+offs), 0)
					s, _ := ps.Token(rdr)
					util.Log(string(s))

					ref := string(util.MakeRef(oo))
					r.rcache[ref] = s
					r.rncache[ref] = -1
				}

			}

			util.Log(o, i)
		}
	}

	r.PageMode = string(r.Dic(r.Trailer["/Root"])["/PageMode"])

	return r
}
