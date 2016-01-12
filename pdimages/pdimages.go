// Extract images from PDF
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/raff/pdfreader/pdfread"
	//"github.com/raff/pdfreader/strm"
	"github.com/raff/pdfreader/util"
)

// The program takes a PDF file and extract the images

func complain(err string) {
	fmt.Printf("%susage: pdimages [--page #] foo.pdf\n", err)
	os.Exit(1)
}

func printdic(dic pdfread.DictionaryT, name, prefix string) {
	fmt.Printf("%s%s {\n", prefix, name)
	for name, value := range dic {
		fmt.Printf("%s  %s: %s\n", prefix, name, string(value))
	}
	fmt.Printf("%s}\n", prefix)
}

const (
	IFD_BYTE     = 1
	IFD_ASCII    = 2
	IFD_SHORT    = 3
	IFD_LONG     = 4
	IFD_RATIONAL = 5

	TAG_IMAGE_WIDTH                = 0x100
	TAG_IMAGE_LENGTH               = 0x101
	TAG_BITS_PER_SAMPLE            = 0x102
	TAG_COMPRESSION                = 0x103
	TAG_PHOTOMETRIC_INTERPRETATION = 0x106
	TAG_STRIP_OFFSETS              = 0x111
	TAG_ROWS_PER_STRIP             = 0x116
	TAG_STRIP_BYTE_COUNTS          = 0x117
	TAG_X_RESOLUTION               = 0x11A
	TAG_Y_RESOLUTION               = 0x11B
	TAG_RESOLUTION_UNIT            = 0x128
)

type IFDEntry struct {
	Tag         uint16
	Type        uint16
	Count       uint32
	ValueOffset uint32
}

type TiffBuilder struct {
	// the output writer
	w io.Writer

	// current offset
	offset uint32

	// entries for current IFD
	ifd []IFDEntry
}

func NewTiffBuilder(w io.Writer) *TiffBuilder {
	return &TiffBuilder{w: w, ifd: []IFDEntry{}}
}

func (t *TiffBuilder) Write(v interface{}) error {
	return binary.Write(t.w, binary.LittleEndian, v)
}

func (t *TiffBuilder) WriteHeader() {
	t.offset = 8

	t.Write([]byte("II"))     // little endian
	t.Write(uint16(42))       // magic
	t.Write(uint32(t.offset)) // offset of first IFD block
}

func (t *TiffBuilder) WriteFooter() {
	t.Write(uint32(0)) // no next IFD
}

func IFDShort(tag uint16, value uint16) IFDEntry {
	return IFDEntry{Tag: tag, Type: IFD_SHORT, Count: 1, ValueOffset: uint32(value)}
}

func IFDLong(tag uint16, value uint32) IFDEntry {
	return IFDEntry{Tag: tag, Type: IFD_LONG, Count: 1, ValueOffset: value}
}

func (t *TiffBuilder) AddShort(tag uint16, value uint16) {
	t.ifd = append(t.ifd, IFDShort(tag, value))
}

func (t *TiffBuilder) AddLong(tag uint16, value uint32) {
	t.ifd = append(t.ifd, IFDLong(tag, value))
}

func (t *TiffBuilder) WriteIFD(data []byte, next bool) {
	util.Logf("offset: %08x\n", t.offset)

	n := len(t.ifd)
	t.offset += 2 + (12 * uint32(n))

	t.Write(uint16(n))

	for _, e := range t.ifd {
		if e.Tag == TAG_STRIP_OFFSETS {
			// assume the only data after the IFD is the image data in one strip
			e.ValueOffset = t.offset
		}

                util.Logf("tag:%v type:%v count:%v value:%v\n", e.Tag, e.Type, e.Count, e.ValueOffset)

		t.Write(e.Tag)
		t.Write(e.Type)
		t.Write(e.Count)

		if e.Count != 1 {
			t.Write(e.ValueOffset)
		} else if e.Type == IFD_LONG {
			t.Write(e.ValueOffset)
		} else if e.Type == IFD_SHORT {
			t.Write(uint16(e.ValueOffset))
			t.Write(uint16(0))
		} else {
			log.Fatal("unsupported type")
		}
	}

	t.ifd = []IFDEntry{}
	t.offset += uint32(len(data)) + 4
	padding := (t.offset & 1) == 1

	if padding {
		t.offset += 1
	}

	if next {
                util.Logf("next:%v\n", t.offset)
		t.Write(uint32(t.offset))
	} else {
                util.Log("next:0")
		t.Write(uint32(0))
	}

        util.Logf("datalen:%v\n", len(data))
	t.w.Write(data)

	if padding {
                util.Log("padding")
		t.w.Write([]byte{0})
	}
}

func extract(pd *pdfread.PdfReaderT, page int, t *TiffBuilder, next bool) {
	pg := pd.Pages()[page-1]
	mbox := util.StringArray(pd.Arr(pd.Att("/MediaBox", pg)))
	fmt.Println("Page", page)
	fmt.Println("  MediaBox", mbox)

	resources := pd.Dic(pd.Att("/Resources", pg))
	if xo := pd.Dic(resources["/XObject"]); xo != nil {
		for name, ref := range xo {
			dic, data := pd.Stream(ref)
			printdic(dic, name, "  ")

			if string(dic["/Subtype"]) != "/Image" {
				continue
			}

			if string(dic["/Filter"]) != "/CCITTFaxDecode" {
				log.Fatal("cannot decode ", string(dic["/Filter"]))
			}

			dparms := pd.Dic(dic["/DecodeParms"])
			cols := pd.Num(dparms["/Columns"])
			rows := pd.Num(dparms["/Rows"])
			k := pd.Num(dparms["/K"])

			if k >= 0 {
				// can't do this right now
				log.Fatal("can't do encoding with K=", k)
			}

			t.AddLong(TAG_IMAGE_WIDTH, uint32(cols))
			t.AddLong(TAG_IMAGE_LENGTH, uint32(rows))
			t.AddShort(TAG_BITS_PER_SAMPLE, 1)
			t.AddShort(TAG_COMPRESSION, 4)
			t.AddShort(TAG_PHOTOMETRIC_INTERPRETATION, 0)
			t.AddLong(TAG_STRIP_OFFSETS, 0)
			t.AddLong(TAG_ROWS_PER_STRIP, uint32(rows))
			t.AddLong(TAG_STRIP_BYTE_COUNTS, uint32(len(data)))

			t.WriteIFD(data, next)
		}
	}
}

func main() {
        flag.BoolVar(&util.Debug, "debug", false, "print debug info")
	page := flag.Int("page", 0, "page to extract, all pages if missing")

	flag.Parse()

	if flag.NArg() != 1 {
		complain("")
	}

	if *page < 0 {
		complain("Bad page!\n\n")
	}

	filename := flag.Arg(0)
	pd := pdfread.Load(filename)
	if pd == nil {
		complain("Could not load pdf file!\n\n")
	}

	npages := len(pd.Pages())

	if *page > npages {
		complain("Page out of range!\n\n")
	}

	output := path.Base(filename)
	if p := strings.LastIndex(output, "."); p > 0 {
		output = output[:p]
	}
	output += ".tif"

	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	t := NewTiffBuilder(f)
	t.WriteHeader()

	if *page == 0 {
		for p := 1; p <= npages; p++ {
			extract(pd, p, t, p < npages)
			fmt.Println("--------------")
		}
		return
	}

	if *page <= npages {
		extract(pd, *page, t, false)
		return
	}
}
