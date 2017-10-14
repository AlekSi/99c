// Copyright 2017 The 99c Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command 99nm lists names in object and executable files produced by the 99c compiler.
//
// Usage
//
//     99nm [files...]
//
// Installation
//
// To install or update 99nm
//
//      $ go get [-u] github.com/cznic/99c/99nm
//
// Online documentation: [godoc.org/github.com/cznic/99c/99nm](http://godoc.org/github.com/cznic/99c/99nm)
//
// Changelog
//
// 2017-10-14: Initial public release.
//
// Sample
//
// Listing names
//
//     $ cd ../examples/nm/
//     $ ls *
//     foo.c
//     $ cat foo.c
//     int i;
//
//     static int j;
//
//     int foo()
//     {
//     }
//
//     static int bar()
//     {
//     }
//
//     int main()
//     {
//     }
//     $ 99c foo.c && 99nm a.out
//     0x00012	__builtin_exit
//     0x0000f	__register_stdfiles
//     0x00000	_start
//     0x00014	main
//     $ 99c -99lib foo.c && 99nm a.out
//     0x00077	__builtin_abort
//     0x00079	__builtin_abs
//     0x0001b	__builtin_alloca
//     0x0001e	__builtin_bswap64
//     0x00021	__builtin_clrsb
//     0x00024	__builtin_clrsbl
//     0x00027	__builtin_clrsbll
//     0x0002a	__builtin_clz
//     0x0002d	__builtin_clzl
//     0x00030	__builtin_clzll
//     0x0005f	__builtin_copysign
//     0x00033	__builtin_ctz
//     0x00036	__builtin_ctzl
//     0x00039	__builtin_ctzll
//     0x00019	__builtin_exit
//     0x00091	__builtin_ffs
//     0x00094	__builtin_ffsl
//     0x00097	__builtin_ffsll
//     0x00068	__builtin_fopen
//     0x0006b	__builtin_fprintf
//     0x0003c	__builtin_frame_address
//     0x00056	__builtin_isprint
//     0x00062	__builtin_longjmp
//     0x00074	__builtin_malloc
//     0x00085	__builtin_memcmp
//     0x0008b	__builtin_memcpy
//     0x0007c	__builtin_memset
//     0x0003f	__builtin_parity
//     0x00042	__builtin_parityl
//     0x00045	__builtin_parityll
//     0x00048	__builtin_popcount
//     0x0004b	__builtin_popcountl
//     0x0004e	__builtin_popcountll
//     0x0006e	__builtin_printf
//     0x00051	__builtin_return_address
//     0x00065	__builtin_setjmp
//     0x00071	__builtin_sprintf
//     0x0008e	__builtin_strchr
//     0x0007f	__builtin_strcmp
//     0x00082	__builtin_strcpy
//     0x00088	__builtin_strlen
//     0x00054	__builtin_trap
//     0x00016	__register_stdfiles
//     0x00059	__signbit
//     0x0005c	__signbitf
//     0x00007	_start
//     0x0009a	foo
//     0x00000	main
//     $ 99c -c foo.c && 99nm foo.o
//     __builtin_abort			func()
//     __builtin_abs			func(int32)int32
//     __builtin_alloca		func(uint64)*struct{}
//     __builtin_bswap64		func(uint64)uint64
//     __builtin_clrsb			func(int32)int32
//     __builtin_clrsbl		func(int64)int32
//     __builtin_clrsbll		func(int64)int32
//     __builtin_clz			func(uint32)int32
//     __builtin_clzl			func(uint64)int32
//     __builtin_clzll			func(uint64)int32
//     __builtin_copysign		func(float64,float64)float64
//     __builtin_ctz			func(uint32)int32
//     __builtin_ctzl			func(uint64)int32
//     __builtin_ctzll			func(uint64)int32
//     __builtin_exit			func(int32)
//     __builtin_ffs			func(int32)int32
//     __builtin_ffsl			func(int64)int32
//     __builtin_ffsll			func(int64)int32
//     __builtin_fopen			func(*int8,*int8)*struct{}
//     __builtin_fprintf		func(*struct{},*int8...)int32
//     __builtin_frame_address		func(uint32)*struct{}
//     __builtin_isprint		func(int32)int32
//     __builtin_longjmp		func(*struct{},int32)
//     __builtin_malloc		func(uint64)*struct{}
//     __builtin_memcmp		func(*struct{},*struct{},uint64)int32
//     __builtin_memcpy		func(*struct{},*struct{},uint64)*struct{}
//     __builtin_memset		func(*struct{},int32,uint64)*struct{}
//     __builtin_parity		func(uint32)int32
//     __builtin_parityl		func(uint64)int32
//     __builtin_parityll		func(uint64)int32
//     __builtin_popcount		func(uint32)int32
//     __builtin_popcountl		func(uint64)int32
//     __builtin_popcountll		func(uint64)int32
//     __builtin_printf		func(*int8...)int32
//     __builtin_return_address	func(uint32)*struct{}
//     __builtin_setjmp		func(*struct{})int32
//     __builtin_sprintf		func(*int8,*int8...)int32
//     __builtin_strchr		func(*int8,int32)*int8
//     __builtin_strcmp		func(*int8,*int8)int32
//     __builtin_strcpy		func(*int8,*int8)*int8
//     __builtin_strlen		func(*int8)uint64
//     __builtin_trap			func()
//     __register_stdfiles		func(*struct{},*struct{},*struct{})
//     __signbit			func(float64)int32
//     __signbitf			func(float32)int32
//     foo				func()int32
//     i				int32
//     main				func()int32
//     $
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/cznic/ir"
	"github.com/cznic/virtual"
	"github.com/cznic/xc"
)

func exit(code int, msg string, arg ...interface{}) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, os.Args[0]+": "+msg, arg...)
	}
	os.Exit(code)
}

const (
	bin = true
	obj = false
)

func use(...interface{}) {}

func main() {
	w := bufio.NewWriter(os.Stdout)

	defer w.Flush()

	for _, arg := range os.Args[1:] {
		if len(os.Args) > 2 {
			fmt.Fprintf(w, "file %v\n", arg)
		}
		switch {
		case filepath.Ext(arg) == ".o":
			use(try(w, arg, obj) || try(w, arg, bin) || unknown(arg))
		default:
			use(try(w, arg, bin) || try(w, arg, obj) || unknown(arg))
		}
	}
}

func unknown(fn string) bool {
	exit(1, "unrecognized file format: %s\n", fn)
	panic("unreachable")
}

func try(w io.Writer, fn string, bin bool) bool {
	tw := new(tabwriter.Writer)
	tw.Init(w, 0, 8, 1, '\t', 0)

	defer tw.Flush()

	w = tw
	f, err := os.Open(fn)
	if err != nil {
		exit(1, "%v\n", err)
	}

	r := bufio.NewReader(f)
	var a []string
	switch {
	case bin:
		var b virtual.Binary
		if _, err := b.ReadFrom(r); err != nil {
			return false
		}

		for k := range b.Sym {
			a = append(a, string(xc.Dict.S(int(k))))
		}
		sort.Strings(a)
		for _, k := range a {
			fmt.Fprintf(w, "%#05x\t%s\n", b.Sym[ir.NameID(xc.Dict.SID(k))], k)
		}
	default:
		var o ir.Objects
		if _, err := o.ReadFrom(r); err != nil {
			return false
		}

		m := map[string][]ir.Object{}
		for _, v := range o {
			k := string(xc.Dict.S(int(v.Base().NameID)))
			a = append(a, k)
			m[k] = append(m[k], v)
		}
		sort.Strings(a)
		for _, k := range a {
			for _, v := range m[k] {
				if v.Base().Linkage == ir.InternalLinkage {
					continue
				}

				switch x := v.(type) {
				case *ir.DataDefinition:
					fmt.Fprintf(w, "%s\t%v\n", k, x.TypeID)
				case *ir.FunctionDefinition:
					fmt.Fprintf(w, "%s\t%v\n", k, x.TypeID)
				}
			}
		}

	}
	return true
}
