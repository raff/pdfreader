// Extract images from PDF
package main

import (
  "fmt"
  "log"
  "os"

  "github.com/yob/pdfreader/pdfread"
  "github.com/yob/pdfreader/strm"
  "github.com/yob/pdfreader/util"
)

// The program takes a PDF file and extract the images

func complain(err string) {
  fmt.Printf("%susage: pdimages foo.pdf [page]\n", err)
  os.Exit(1)
}

func extract(pd *pdfread.PdfReaderT, page int) {
  pg := pd.Pages()[page-1]
  mbox := util.StringArray(pd.Arr(pd.Att("/MediaBox", pg)))
  log.Println("Page", page)
  log.Println("/MediaBox", mbox)

  resources := pd.Dic(pd.Att("/Resources", pg))
  if xo := pd.Dic(resources["/XObject"]); xo != nil {
    for name, ref := range(xo) {
        log.Println(name, string(ref))

        dic, _ := pd.Stream(ref)
        log.Println(dic)
    }
  }
}

func main() {
  if len(os.Args) == 1 || len(os.Args) > 3 {
    complain("")
  }
  page := 0
  if len(os.Args) > 2 {
    page = strm.Int(os.Args[2], 1)
    if page < 0 {
      complain("Bad page!\n\n")
    }
  }
  pd := pdfread.Load(os.Args[1])
  if pd == nil {
    complain("Could not load pdf file!\n\n")
  }

  npages := len(pd.Pages())
  
  if page == 0 {
    for page = 1; page <= npages; page++ {
        extract(pd, page)
        fmt.Println("--------------")
    }
    return
  }

  if page <= npages {
    extract(pd, page)
    return
  }

  complain("Page out of range!\n\n")
}
