// Copyright (c) 2009 Helmar Wodtke. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// The MIT License is an OSI approved license and can
// be found at
//   http://www.opensource.org/licenses/mit-license.php

// Some utilities.
package util

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/raff/pdfreader/xchar"
)

var (
	Debug = false

	wrongUniCode = xchar.Utf8(-1)
	hexRE        = regexp.MustCompile(`#[A-F0-9]{2}`)
)

// util.Bytes() is a dup of string.Bytes()
func Bytes(a string) []byte {
	r := make([]byte, len(a))
	for k := 0; k < len(a); k++ {
		r[k] = byte(a[k])
	}
	return r
}

// util.Unescape() convert #XX in input back to char
func Unescape(b []byte) string {
	return hexRE.ReplaceAllStringFunc(string(b), func(p string) string {
		v, _ := strconv.ParseUint(p[1:], 16, 8)
		return string([]byte{byte(v)})
	})
}

func ApplyPNGPredictor(pred, colors, columns, bitspercomponent int, data []byte) []byte {
	if bitspercomponent != 8 {
		// unsupported
		Log("Unsupported bitspercomponent", bitspercomponent)
		return nil
	}

	nbytes := colors * columns * bitspercomponent / 8
	buf := []byte{}

	line0 := bytes.Repeat([]byte{0}, columns)

	for i := 0; i < len(data); i += nbytes {
		ft := data[i]

		i += 1

		line1 := data[i : i+nbytes]
		line2 := []byte{}

		switch ft {
		case 0:
			// PNG none
			line2 = append(line2, line1...)

		case 1:
			// PNG sub (UNTESTED)
			c := byte(0)

			for _, b := range line1 {
				c = (c + b) & 255
				line2 = append(line2, c)
			}

		case 2:
			// PNG up
			l := len(line0)
			if len(line1) < l {
				l = len(line1)
			}

			for i := 0; i < l; i++ {
				a, b := line0[i], line1[i]
				c := (a + b) & 255
				line2 = append(line2, c)
			}

		case 3:
			// PNG average (UNTESTED)
			c := byte(0)

			l := len(line0)
			if len(line1) < l {
				l = len(line1)
			}

			for i := 0; i < l; i++ {
				a, b := line0[i], line1[i]
				c := ((c + a + b) / 2) & 255
				line2 = append(line2, c)
			}

		default:
			// unsupported
			Log("Unsupported predictor (ft)", ft)
			return nil
		}

		buf = append(buf, line2...)
		line0 = line2
	}

	return buf
}

func MakeRef(o int) []byte {
	return []byte(fmt.Sprintf("%d 0 R", o))
}

func JoinStrings(a []string, c byte) []byte {
	if a == nil || len(a) == 0 {
		return []byte{}
	}
	l := 0
	for k := range a {
		l += len(a[k]) + 1
	}
	r := make([]byte, l)
	q := 0
	for k := range a {
		for i := 0; i < len(a[k]); i++ {
			r[q] = a[k][i]
			q++
		}
		r[q] = c
		q++
	}
	return r[0 : l-1]
}

func StringArray(i [][]byte) []string {
	r := make([]string, len(i))
	for k := range i {
		r[k] = string(i[k])
	}
	return r
}

func set(o []byte, q string) int {
	for k := 0; k < len(q); k++ {
		o[k] = q[k]
	}
	return len(q)
}

func ToXML(s []byte) []byte {
	l := len(s)
	for k := range s {
		switch s[k] {
		case '<', '>':
			l += 3
		case '&':
			l += 4
		case 0, 1, 2, 3, 4, 5, 6, 7, 8,
			11, 12, 14, 15, 16, 17, 18, 19, 20,
			21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
			31:
			l += len(wrongUniCode) - 1
		}
	}
	r := make([]byte, l)
	p := 0
	for k := range s {
		switch s[k] {
		case '<':
			p += set(r[p:p+4], "&lt;")
		case '>':
			p += set(r[p:p+4], "&gt;")
		case '&':
			p += set(r[p:p+5], "&amp;")
		case 10, 9, 13:
			r[p] = s[k]
			p++
		default:
			if s[k] < 32 {
				p += copy(r[p:], wrongUniCode)
			} else {
				r[p] = s[k]
				p++
			}
		}
	}
	return r
}

type OutT struct {
	Content []byte
}

func (t *OutT) Out(f string, args ...interface{}) {
	p := fmt.Sprintf(f, args...)
	q := len(t.Content)
	if cap(t.Content)-q < len(p) {
		n := make([]byte, cap(t.Content)+(len(p)/512+2)*512)
		copy(n, t.Content)
		t.Content = n[0:q]
	}
	t.Content = t.Content[0 : q+len(p)]
	for k := 0; k < len(p); k++ {
		t.Content[q+k] = p[k]
	}
}

func IsHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func Log(args ...interface{}) {
	if Debug {
		log.Println(args...)
	}
}

func Logf(f string, args ...interface{}) {
	if Debug {
		log.Printf(f, args...)
	}
}
