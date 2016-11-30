package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/desal/git"
	"github.com/desal/gocmd"
	"github.com/desal/richtext"
)

//go:generate stringer -type Flag

type (
	empty     struct{}
	Flag      int
	flagSet   map[Flag]empty
	stringSet map[string]empty

	Context struct {
		startPkg string
		doneDirs stringSet
		format   richtext.Format
		goPath   []string
		goCtx    *gocmd.Context
		gitCtx   *git.Context
		gitFlags []git.Flag
		flags    flagSet
	}

	DepsFile struct {
		Deps     []PkgDep
		TestDeps []PkgDep
	}

	PkgDep struct {
		ImportPath string
		GitRemote  string //Blank for standard packages
		SHA        string //Blank for standard packages
		Tags       []string
		Error      error `json:"-"`
	}

	PkgDepsByImport []PkgDep
)

const (
	_          Flag = iota
	MustExit        //
	MustPanic       //
	Warn            //
	Verbose         // show pkgname\n as it goes
	CmdVerbose      // Also displays commands being executed
	SkipVendor      //
)

var (
	gitFlags = map[Flag]git.Flag{
		MustExit:   git.MustExit,
		MustPanic:  git.MustPanic,
		Warn:       git.Warn,
		CmdVerbose: git.Verbose,
	}
)

func (fs flagSet) Checked(flag Flag) bool {
	_, ok := fs[flag]

	return ok
}

func (ss stringSet) Sorted() []string {
	r := []string{}
	for s, _ := range ss {
		r = append(r, s)
	}
	sort.Strings(r)
	return r
}

func (d *DepsFile) Sort() {
	sort.Sort(PkgDepsByImport(d.Deps))
	sort.Sort(PkgDepsByImport(d.TestDeps))
}

func (a PkgDepsByImport) Len() int           { return len(a) }
func (a PkgDepsByImport) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PkgDepsByImport) Less(i, j int) bool { return a[i].ImportPath < a[j].ImportPath }

func New(format richtext.Format, goPath []string, flags ...Flag) *Context {
	c := &Context{
		doneDirs: stringSet{},
		format:   format,
		goPath:   goPath,
		goCtx:    gocmd.New(format, goPath, ""),
		flags:    flagSet{},
	}

	for _, flag := range flags {
		if gitFlag, ok := gitFlags[flag]; ok {
			c.gitFlags = append(c.gitFlags, gitFlag)
		}
		c.flags[flag] = empty{}
	}

	c.gitCtx = git.New(format, c.gitFlags...)

	return c
}

func (c *Context) errorf(s string, a ...interface{}) error {
	if c.flags.Checked(MustExit) {
		c.format.ErrorLine(s, a...)
		os.Exit(1)
	} else if c.flags.Checked(MustPanic) {
		panic(fmt.Errorf(s, a...))
	} else if c.flags.Checked(Warn) || c.flags.Checked(Verbose) {
		c.format.WarningLine(s, a...)
	}
	return fmt.Errorf(s, a...)
}

func (c *Context) warnf(s string, a ...interface{}) {
	if c.flags.Checked(Warn) {
		c.format.WarningLine(s, a...)
	}
}

func (c *Context) verbosef(s string, a ...interface{}) {
	if c.flags.Checked(Verbose) {
		c.format.PrintLine(s, a...)
	}
}

func ReadJson(filename string) (DepsFile, error) {
	var result DepsFile

	var r io.Reader
	var err error
	if filename == "stdin" {
		r = os.Stdin
	} else {
		r, err = os.Open(filename)
		if err != nil {
			return result, err
		}
	}

	dec := json.NewDecoder(r)
	err = dec.Decode(&result)
	return result, err
}

func WriteJson(filename string, depsFile DepsFile) error {
	jsonOutput, err := json.MarshalIndent(&depsFile, "", "  ")
	if err != nil {
		return err
	}

	if filename == "stdout" {
		_, err := os.Stdout.Write(jsonOutput)
		return err
	} else {
		return ioutil.WriteFile(filename, jsonOutput, 0644)
	}
}

func pkgContains(parent, child string) bool {
	if parent == child || strings.HasPrefix(child, parent+"/") {
		return true
	}
	return false
}
