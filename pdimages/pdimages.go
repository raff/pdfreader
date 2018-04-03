// Extract images from PDF
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/raff/pdfreader/fancy"
	"github.com/raff/pdfreader/pdfread"
	"github.com/raff/pdfreader/util"

	tiff "github.com/chai2010/tiff"
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

	TAG_IMAGE_WIDTH                = 0x100 // 256
	TAG_IMAGE_LENGTH               = 0x101 // 257
	TAG_BITS_PER_SAMPLE            = 0x102 // 258
	TAG_COMPRESSION                = 0x103 // 259
	TAG_PHOTOMETRIC_INTERPRETATION = 0x106 // 262
	TAG_STRIP_OFFSETS              = 0x111 // 273
	TAG_ORIENTATION                = 0x112 // 274
	TAG_SAMPLES_PER_PIXEL          = 0x115 // 277
	TAG_ROWS_PER_STRIP             = 0x116 // 278
	TAG_STRIP_BYTE_COUNTS          = 0x117 // 279
	TAG_X_RESOLUTION               = 0x11A // 282
	TAG_Y_RESOLUTION               = 0x11B // 283
	TAG_RESOLUTION_UNIT            = 0x128 // 296
)

type IFDEntry struct {
	Tag         uint16
	Type        uint16
	Count       uint32
	ValueOffset uint32

	Num, Den uint32 // used only for IFDRational
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

func (t *TiffBuilder) WriteBytes(b []byte) error {
	_, err := t.w.Write(b)
	return err
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

func IFDRational(tag uint16, num, den uint32) IFDEntry {
	return IFDEntry{Tag: tag, Type: IFD_RATIONAL, Count: 1, ValueOffset: 0, Num: num, Den: den}
}

func (t *TiffBuilder) AddShort(tag uint16, value uint16) {
	t.ifd = append(t.ifd, IFDShort(tag, value))
}

func (t *TiffBuilder) AddLong(tag uint16, value uint32) {
	t.ifd = append(t.ifd, IFDLong(tag, value))
}

func (t *TiffBuilder) AddRational(tag uint16, num, den uint32) {
	t.ifd = append(t.ifd, IFDRational(tag, num, den))
}

func (t *TiffBuilder) WriteIFD(data []byte, next bool) {
	util.Logf("offset: %08x\n", t.offset)

	n := len(t.ifd)
	t.offset += 6 + (12 * uint32(n))

	t.Write(uint16(n))

	padding := false

	for _, e := range t.ifd {
		if e.Tag == TAG_STRIP_OFFSETS || e.Type == IFD_RATIONAL {
			e.ValueOffset = t.offset

			if e.Tag == TAG_STRIP_OFFSETS {
				util.Log("offset:", t.offset)

				t.offset += uint32(len(data)) + 4
				padding = (t.offset & 1) == 1
				if padding {
					t.offset += 1
				}
			} else {
				util.Log("offset:", t.offset)
				t.offset += 8 // 4 + 4
			}
		}

		util.Logf("tag:%v type:%v count:%v value:%v\n", e.Tag, e.Type, e.Count, e.ValueOffset)

		t.Write(e.Tag)
		t.Write(e.Type)
		t.Write(e.Count)

		if e.Count != 1 {
			t.Write(e.ValueOffset)
		} else if e.Type == IFD_LONG {
			t.Write(e.ValueOffset)
		} else if e.Type == IFD_RATIONAL {
			t.Write(e.ValueOffset)
		} else if e.Type == IFD_SHORT {
			t.Write(uint16(e.ValueOffset))
			t.Write(uint16(0))
		} else {
			log.Fatal("unsupported type")
		}
	}

	if next {
		util.Log("next:", t.offset)
		t.Write(uint32(t.offset))
	} else {
		util.Log("next:0")
		t.Write(uint32(0))
	}

	util.Log("datalen:", len(data))
	t.WriteBytes(data)

	if padding {
		t.WriteBytes([]byte{0})
		util.Log("padding:", t.offset)
	}

	for _, e := range t.ifd {
		if e.Type != IFD_RATIONAL {
			continue
		}

		t.Write(e.Num)
		t.Write(e.Den)
	}

	t.ifd = []IFDEntry{}
}

func extract(pd *pdfread.PdfReaderT, page int, base string) (filename string, err error) {
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

			switch string(dic["/Filter"]) {
			case "/CCITTFaxDecode": // TIFF
				if string(dic["/ColorSpace"]) != "/DeviceGray" {
					log.Fatal("cannot convert /CCITTFaxDecode ", string(pd.Obj(dic["/ColorSpace"])))
				}

				dparms := pd.Dic(dic["/DecodeParms"])
				width := pd.Num(dparms["/Columns"])
				height := pd.Num(dparms["/Rows"])
				k := pd.Num(dparms["/K"])
				bpc := pd.Num(dic["/BitsPerComponent"])

				if k >= 0 {
					// can't do this right now
					log.Fatal("can't do encoding with K=", k)
				}

				buffer := &bytes.Buffer{}

				t := NewTiffBuilder(buffer)
				t.WriteHeader()
				t.AddLong(TAG_IMAGE_WIDTH, uint32(width))
				t.AddLong(TAG_IMAGE_LENGTH, uint32(height))
				t.AddShort(TAG_BITS_PER_SAMPLE, uint16(bpc))
				t.AddShort(TAG_COMPRESSION, 4)                // CCITT Group 4
				t.AddShort(TAG_PHOTOMETRIC_INTERPRETATION, 0) // white is zero
				t.AddLong(TAG_STRIP_OFFSETS, 0)
				//t.AddShort(TAG_ORIENTATION, 1)
				//t.AddShort(TAG_SAMPLES_PER_PIXEL, 1)
				t.AddLong(TAG_ROWS_PER_STRIP, uint32(height))
				t.AddLong(TAG_STRIP_BYTE_COUNTS, uint32(len(data)))
				//t.AddRational(TAG_X_RESOLUTION, 300, 1) // 300 dpi (300/1)
				//t.AddRational(TAG_Y_RESOLUTION, 300, 1) // 300 dpi (300/1)
				//t.AddShort(TAG_RESOLUTION_UNIT, 2)      // pixels/inch
				t.WriteIFD(data, false)

				buffer = bytes.NewBuffer(buffer.Bytes())
				ima, derr := tiff.Decode(buffer)
				if derr != nil {
					return "", derr
				}

				filename = base + ".png"

				f, cerr := os.Create(filename)
				if cerr != nil {
					return "", cerr
				}

				err = png.Encode(f, ima)
				f.Close()
				return

			case "/DCTDecode": // JPEG
				/*
					width := pd.Num(dic["/Width"])
					height := pd.Num(dic["/Height"])
					bpc := pd.Num(dic["/BitsPerComponent"])
				*/

				filename = base + ".jpg"

				f, cerr := os.Create(filename)
				if cerr != nil {
					return "", cerr
				}

				_, err = f.Write(data)
				f.Close()
				return

			case "/FlateDecode": // compressed bitmap
				data = fancy.ReadAndClose(zlib.NewReader(fancy.SliceReader(data)))
				width := pd.Num(dic["/Width"])
				height := pd.Num(dic["/Height"])
				bpc := pd.Num(dic["/BitsPerComponent"])

				if bpc != 8 {
					log.Fatal("cannot convert /FlateDecode bpc:", bpc)
				}

				if string(dic["/ColorSpace"]) != "/DeviceRGB" {
					log.Fatal("cannot convert /FlateDecode ", string(pd.Obj(dic["/ColorSpace"])))
				}

				ima := image.NewRGBA(image.Rect(0, 0, width, height))

				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						ima.Set(x, y, color.RGBA{R: data[0], G: data[1], B: data[2], A: 255})
						data = data[3:]
					}
				}

				filename = base + ".png"

				f, cerr := os.Create(filename)
				if cerr != nil {
					return "", cerr
				}

				err = png.Encode(f, ima)
				f.Close()
				return

			default:
				log.Fatal("cannot decode ", string(dic["/Filter"]))
			}
		}
	}

	return
}

func main() {
	flag.BoolVar(&util.Debug, "debug", false, "print debug info")
	page := flag.Int("page", 0, "page to extract, all pages if missing")

	flag.Parse()

	if flag.NArg() != 1 {
		complain("")
	}

	filename := flag.Arg(0)
	pd := pdfread.Load(filename)
	if pd == nil {
		complain("Could not load pdf file!\n\n")
	}

	npages := len(pd.Pages())

	if *page < 0 {
		*page += npages + 1
	}

	if *page < 0 || *page > npages {
		complain("Page out of range!\n\n")
	}

	output := path.Base(filename)
	if p := strings.LastIndex(output, "."); p > 0 {
		output = output[:p]
	}

	if *page == 0 {
		for p := 1; p <= npages; p++ {
			pout := fmt.Sprintf("%s_%d", output, p)
			if _, err := extract(pd, p, pout); err != nil {
				log.Println("error extracting page", p, err)
				break
			}
			fmt.Println("--------------")
		}
	} else if *page <= npages {
		if _, err := extract(pd, *page, output); err != nil {
			log.Println("error extracting page", *page, err)
		}
	}
}
