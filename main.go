// Copyright 2017 The 99c Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go get -u
//go:generate go install -tags virtual.profile ./99prof
//go:generate go install -tags virtual.strace ./99strace
//go:generate go install -tags virtual.trace ./99trace
//go:generate go install ./99dump ./99nm ./99run
package main

import (
	"bufio"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/cznic/cc"
	"github.com/cznic/ccir"
	"github.com/cznic/ir"
	"github.com/cznic/strutil"
	"github.com/cznic/virtual"
	"github.com/cznic/xc"
)

func exit(code int, msg string, arg ...interface{}) {
	msg = strings.TrimSpace(msg)
	if msg != "" {
		fmt.Fprintf(os.Stderr, os.Args[0]+": "+msg+"\n", arg...)
	}
	os.Stderr.Sync()
	os.Exit(code)
}

func main() {
	if s := os.Getenv("DIAG99C"); strings.Contains(","+s+",", ",os-args,") {
		fmt.Fprintf(os.Stderr, "%v\n", os.Args)
	}

	defer func() {
		if err := recover(); err != nil {
			exit(1, "PANIC: %v\n%s", err, debug.Stack())
		}
	}()

	t := newTask()
	t.args.getopt(os.Args)
	if err := t.main(); err != nil {
		switch x := err.(type) {
		case scanner.ErrorList:
			scanner.PrintError(os.Stderr, x)
			os.Exit(1)
		default:
			exit(1, "%v", err)
		}
	}
}

type testHooks struct {
	bin **virtual.Binary
	obj *ir.Objects
}

type args struct {
	D      []string // -D
	E      bool     // -E
	I      []string // -I
	L      []string // -L
	O      string   // -O
	W      string   // -W
	args   []string // Non flag arguments in order of appearance.
	c      bool     // -c
	g      bool     // -g
	hooks  testHooks
	l      []string // -l
	lib    bool     // -99lib
	o      string   // -o
	opts   []cc.Opt // cc flags
	shared bool     // -shared
}

func (a *args) extra(name string) cc.Opt {
	switch name {
	case "AlignOf":
		return cc.EnableAlignOf()
	case "AlternateKeywords":
		return cc.EnableAlternateKeywords()
	case "AnonymousStructFields":
		return cc.EnableAnonymousStructFields()
	case "Asm":
		return cc.EnableAsm()
	case "BuiltinClassifyType":
		return cc.EnableBuiltinClassifyType()
	case "BuiltinConstantP":
		return cc.EnableBuiltinConstantP()
	case "ComputedGotos":
		return cc.EnableComputedGotos()
	case "DefineOmitCommaBeforeDDD":
		return cc.EnableDefineOmitCommaBeforeDDD()
	case "DlrInIdentifiers":
		return cc.EnableDlrInIdentifiers()
	case "EmptyDeclarations":
		return cc.EnableEmptyDeclarations()
	case "EmptyDefine":
		return cc.EnableEmptyDefine()
	case "EmptyStructs":
		return cc.EnableEmptyStructs()
	case "ImaginarySuffix":
		return cc.EnableImaginarySuffix()
	case "ImplicitFuncDef":
		return cc.EnableImplicitFuncDef()
	case "ImplicitIntType":
		return cc.EnableImplicitIntType()
	case "IncludeNext":
		return cc.EnableIncludeNext()
	case "LegacyDesignators":
		return cc.EnableLegacyDesignators()
	case "NonConstStaticInitExpressions":
		return cc.EnableNonConstStaticInitExpressions()
	case "Noreturn":
		return cc.EnableNoreturn()
	case "OmitConditionalOperand":
		return cc.EnableOmitConditionalOperand()
	case "OmitFuncArgTypes":
		return cc.EnableOmitFuncArgTypes()
	case "OmitFuncRetType":
		return cc.EnableOmitFuncRetType()
	case "ParenthesizedCompoundStatemen":
		return cc.EnableParenthesizedCompoundStatemen()
	case "StaticAssert":
		return cc.EnableStaticAssert()
	case "TypeOf":
		return cc.EnableTypeOf()
	case "UndefExtraTokens":
		return cc.EnableUndefExtraTokens()
	case "UnsignedEnums":
		return cc.EnableUnsignedEnums()
	case "WideBitFieldTypes":
		return cc.EnableWideBitFieldTypes()
	case "WideEnumValues":
		return cc.EnableWideEnumValues()
	}
	exit(2, "unknown -99extra argument: %s", name)
	panic("unreachable")
}

func (a *args) getopt(args []string) {
	args = args[1:]
	for i, arg := range args {
		switch {
		case arg == "-99lib":
			a.lib = true
		case strings.HasPrefix(arg, "-D"):
			if arg == "-D" {
				break
			}

			arg = arg[2:]
			p := strings.SplitN(arg, "=", 2)
			if len(p) == 1 {
				p = append(p, "1")
			}
			a.D = append(a.D, fmt.Sprintf("#define %s %s", p[0], p[1]))
		case arg == "-E":
			a.E = true
		case strings.HasPrefix(arg, "-I"):
			if arg == "-I" {
				break
			}

			arg = arg[2:]
			a.I = append(a.I, arg)
		case strings.HasPrefix(arg, "-L"):
			if arg == "-L" {
				break
			}

			arg = arg[2:]
			a.L = append(a.L, arg)
		case strings.HasPrefix(arg, "-O"):
			a.O = arg[2:]
		case strings.HasPrefix(arg, "-W"):
			a.W = arg[2:]
		case arg == "-ansi":
			// nop
		case arg == "-c":
			a.c = true
		case arg == "-99extra":
			if i+1 >= len(args) {
				exit(2, "missing -99extra argument")
			}

			a.opts = append(a.opts, a.extra(args[i+1]))
			args[i+1] = ""
		case arg == "-g":
			a.g = true
		case strings.HasPrefix(arg, "-l"):
			if arg == "-l" {
				break
			}

			arg = arg[2:]
			a.l = append(a.l, arg)
		case arg == "-o":
			if i+1 >= len(args) {
				exit(2, "missing -o argument")
			}

			a.o = args[i+1]
			args[i+1] = ""
		case arg == "-pedantic":
			// nop
		case arg == "-pthread":
			//TODO
		case arg == "-rpath":
			if i+1 >= len(args) {
				exit(2, "missing -rptah argument")
			}

			a.o = args[i+1]
			args[i+1] = ""
		case arg == "-shared":
			a.shared = true
			//TODO
		case arg == "-soname":
			if i+1 >= len(args) {
				exit(2, "missing -soname argument")
			}

			//TODO
			args[i+1] = ""
		case strings.HasPrefix(arg, "-"):
			s := ""
			if arg != "-h" {
				s = fmt.Sprintf("%v unknown flag: %s\n", os.Args, arg)
			}
			exit(2, `%sFlags:
  -99lib
        Library link mode.
  -Dname
        Equivalent to inserting '#define name 1' at the start of the
        translation unit.
  -Dname=definition
        Equivalent to inserting '#define name definition' at the start of the
        translation unit.
  -E    Copy C-language source files to standard output, executing all
        preprocessor directives; no compilation shall be performed. If any
        operand is not a text file, the effects are unspecified.
  -Ipath
        Add path to the include files search paths.
  -Lpath
        Add path to search paths for -l.
  -Olevel
        Optimization setting, ignored.
  -Wwarn
        Warning level, ignored.
  -ansi
        Ignored.
  -c    Suppress the link-edit phase of the compilation, and do not
        remove any object files that are produced.
  -g    Produce debugging information, ignored.
  -l<name>
        Link with lib<name>.
  -o pathname
        Use the specified pathname, instead of the default a.out, for
        the executable file produced. If the -o option is present with
        -c or -E, the result is unspecified.
  -pedantic
        Ignored.
  -pthread
        Ignored. (TODO)
  -rpath pathname
        Ignored. (TODO)
  -shared
        Link mode shared library.
  -soname arg
        Ignored. (TODO)
  -99extra flag
     Extra cc flags:
        AlignOf
        AlternateKeywords
        AnonymousStructFields
        Asm
        BuiltinClassifyType
        BuiltinConstantP
        ComputedGotos
        DefineOmitCommaBeforeDDD
        DlrInIdentifiers
        EmptyDeclarations
        EmptyDefine
        EmptyStructs
        ImaginarySuffix
        ImplicitFuncDef
        ImplicitIntType
        IncludeNext
        LegacyDesignators
        NonConstStaticInitExpressions
        Noreturn
        OmitConditionalOperand
        OmitFuncArgTypes
        OmitFuncRetType
        ParenthesizedCompoundStatemen
        StaticAssert
        TypeOf
        UndefExtraTokens
        UnsignedEnums
        WideBitFieldTypes
        WideEnumValues`,
				s)
		default:
			if arg != "" {
				a.args = append(a.args, arg)
			}
		}
	}
}

type task struct {
	args   args
	cfiles []string
	ofiles []string
}

func fatalError(msg string, arg ...interface{}) error {
	return fmt.Errorf("fatal error: %s", fmt.Sprintf(msg, arg...))
}

func newTask() *task { return &task{} }

func (t *task) main() error {
	if h := strutil.Homepath(); h != "" {
		p := filepath.Join(h, ".99c")
		fi, err := os.Stat(p)
		if err == nil {
			if fi.IsDir() {
				t.args.I = append(t.args.I, filepath.Join(p, "include"))
				t.args.L = append(t.args.I, filepath.Join(p, "lib"))
			}
		}
	}

	if len(t.args.args) == 0 {
		return fatalError("no input files")
	}

	if t.args.o != "" && (t.args.c || t.args.E) && len(t.args.args) > 1 {
		exit(2, "cannot specify -o with -c or -E with multiple files")
	}

	lsearch := append([]string{"."}, t.args.L...)
	for _, v := range t.args.l {
		for _, d := range lsearch {
			fn := filepath.Join(d, fmt.Sprintf("lib%s.so", v))
			_, err := os.Stat(fn)
			if err != nil {
				if !os.IsNotExist(err) {
					return fatalError("%v", err)
				}

				continue
			}

			t.ofiles = append(t.ofiles, fn)
		}
	}

	for _, arg := range t.args.args {
		switch filepath.Ext(arg) {
		case ".c", ".h":
			t.cfiles = append(t.cfiles, arg)
		case ".o", ".so":
			t.ofiles = append(t.ofiles, arg)
		default:
			return fatalError("unrecognized file type: %v", arg)
		}
	}

	switch {
	case t.args.E:
		model, err := ccir.NewModel()
		if err != nil {
			fatalError("%v", err)
		}

		o := os.Stdout
		if fn := t.args.o; fn != "" {
			if o, err = os.Create(fn); err != nil {
				fatalError("%v", err)
			}
		}
		out := bufio.NewWriter(o)

		defer out.Flush()

		var lpos token.Position
		opts := []cc.Opt{
			cc.IncludePaths(t.args.I),
			cc.SysIncludePaths([]string{ccir.LibcIncludePath}),
			cc.AllowCompatibleTypedefRedefinitions(),
			cc.Cpp(func(toks []xc.Token) {
				if len(toks) != 0 {
					p := toks[0].Position()
					if p.Filename != lpos.Filename {
						fmt.Fprintf(out, "# %d %q\n", p.Line, p.Filename)
					}
					lpos = p
				}
				for _, v := range toks {
					if s := strings.TrimSpace(cc.TokSrc(v)); s != "" {
						fmt.Fprintf(out, "%v ", s)
					}
				}
				fmt.Fprintln(out)
			}),
		}
		opts = append(opts, t.args.opts...)

		_, err = cc.Parse(
			fmt.Sprintf(`
%s
#define __arch__ %s
#define __os__ %s
#include <builtin.h>
`, strings.Join(t.args.D, "\n"), runtime.GOARCH, runtime.GOOS),
			t.cfiles,
			model,
			opts...,
		)
		return err
	}

	var obj ir.Objects
	for _, fn := range t.ofiles {
		f, err := os.Open(fn)
		if err != nil {
			return fatalError("%v", err)
		}

		r := bufio.NewReader(f)
		var o ir.Objects
		if _, err := o.ReadFrom(r); err != nil {
			return fatalError("%v", err)
		}

		obj = append(obj, o...)
	}

	switch {
	case t.args.c:
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		for _, arg := range t.cfiles {
			model, err := ccir.NewModel()
			if err != nil {
				fatalError("%v", err)
			}

			opts := []cc.Opt{
				cc.IncludePaths(t.args.I),
				cc.SysIncludePaths([]string{ccir.LibcIncludePath}),
				cc.AllowCompatibleTypedefRedefinitions(),
			}
			opts = append(opts, t.args.opts...)

			tu, err := cc.Parse(
				fmt.Sprintf(`
%s
#define __arch__ %s
#define __os__ %s
#include <builtin.h>
`, strings.Join(t.args.D, "\n"), runtime.GOARCH, runtime.GOOS),
				[]string{arg},
				model,
				opts...,
			)
			if err != nil {
				return err
			}

			o, err := ccir.New(tu)
			if err != nil {
				return err
			}

			if p := t.args.hooks.obj; p != nil {
				*p = ir.Objects{o}
			}
			fn := filepath.Join(wd, filepath.Base(arg[:len(arg)-len(filepath.Ext(arg))])+".o")
			f, err := os.Create(fn)
			if err != nil {
				return err
			}

			w := bufio.NewWriter(f)
			if _, err := (ir.Objects{o}).WriteTo(w); err != nil {
				return err
			}

			if err := w.Flush(); err != nil {
				return err
			}

			if err := f.Close(); err != nil {
				return err
			}
		}
		return nil
	default:
		fn := t.args.o
		if fn == "" {
			fn = "a.out"
		}
		model, err := ccir.NewModel()
		if err != nil {
			fatalError("%v", err)
		}

		opts := []cc.Opt{
			cc.IncludePaths(t.args.I),
			cc.SysIncludePaths([]string{ccir.LibcIncludePath}),
			cc.AllowCompatibleTypedefRedefinitions(),
		}
		opts = append(opts, t.args.opts...)

		tu, err := cc.Parse(
			fmt.Sprintf(`
%s
#define __arch__ %s
#define __os__ %s
#include <builtin.h>
`, strings.Join(t.args.D, "\n"), runtime.GOARCH, runtime.GOOS),
			append(t.cfiles, ccir.CRT0Path),
			model,
			opts...,
		)
		if err != nil {
			return err
		}

		o, err := ccir.New(tu)
		if err != nil {
			return err
		}

		var out ir.Objects
		in := append(obj, o)
		switch {
		case t.args.shared:
			out = append(out, in...)
			f, err := os.Create(fn)
			if err != nil {
				return err
			}

			_, err = out.WriteTo(f)
			return err
		case t.args.lib:
			o, err = ir.LinkLib(in...)
			out = ir.Objects{o}
		default:
			for _, v := range in {
				for _, o := range v {
					if err := o.Verify(); err != nil {
						return err
					}
				}
			}
			o, err = ir.LinkMain(in...)
			out = ir.Objects{o}
		}
		if err != nil {
			return err
		}

		if len(out) != 1 {
			panic("internal error")
		}

		bin, err := virtual.LoadMain(out[0])
		if err != nil {
			return err
		}

		if p := t.args.hooks.bin; p != nil {
			*p = bin
		}
		f, err := os.Create(fn)
		if err != nil {
			return err
		}

		if runtime.GOOS == "linux" {
			f.WriteString("#!/usr/bin/env 99run\n")
		}

		if _, err := bin.WriteTo(f); err != nil {
			return err
		}

		if err := f.Close(); err != nil {
			return err
		}

		fi, err := os.Stat(fn)
		if err != nil {
			return err
		}

		m := fi.Mode()
		for k, b := os.FileMode(0400), os.FileMode(0100); k != 0; k, b = k>>3, b>>3 {
			if m&k != 0 {
				m |= b
			}
		}

		return os.Chmod(fn, m)
	}
}
