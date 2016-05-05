package snapshot

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/desal/cmd"
	"github.com/desal/dsutil"
	"github.com/desal/git"
	"github.com/desal/gocmd"
)

type empty struct{}
type set map[string]empty

type Context struct {
	startPkg string
	doneDirs set
	output   cmd.Output
	goPath   []string
	goCtx    *gocmd.Context
	gitCtx   *git.Context
}

type DepsFile struct {
	Deps     []PkgDep
	TestDeps []PkgDep
}

type PkgDep struct {
	ImportPath string
	GitRemote  string //Blank for standard packages
	SHA        string //Blank for standard packages
}

func NewContext(output cmd.Output, goPath []string) *Context {
	return &Context{
		doneDirs: set{},
		output:   output,
		goPath:   goPath,
		goCtx:    gocmd.New(output, goPath),
		gitCtx:   git.New(output),
	}
}

func pkgContains(parent, child string) bool {
	if parent == child || strings.HasPrefix(child, parent+"/") {
		return true
	}
	return false
}

func (c *Context) scanDeps(startingList map[string]map[string]interface{}, workingDir string, deps set) []PkgDep {
	result := []PkgDep{}
outer:
	for importPath, _ := range deps {
		if c.goCtx.IsStdLib(importPath) {
			continue
		}

		if _, isStartingPkg := startingList[importPath]; isStartingPkg {
			continue
		}

		list, _ := c.goCtx.List(workingDir, importPath)

		cmdCtx := cmd.NewContext(".", c.output, cmd.Must)
		dir, _ := dsutil.SanitisePath(cmdCtx, list[importPath]["Dir"].(string))

		if !strings.HasSuffix(dir, importPath) {
			c.output.Error("Package import path directory mismatch (path sanity failure) %s should end in %s", dir, importPath)
			os.Exit(1)
		}

		if _, done := c.doneDirs[dir]; done {
			continue
		}

		for doneDir, _ := range c.doneDirs {
			if strings.HasPrefix(dir, doneDir+"/") {
				continue outer
			}
		}

		if !c.gitCtx.IsGit(dir) {
			c.output.Error("Import %s (%s) is not a git repository", importPath, dir)
			os.Exit(1)
		}

		status, _ := c.gitCtx.Status(dir, true)
		if status != git.Clean {
			c.output.Error("Import %s (%s) has git status %s", importPath, dir, status.String())
			os.Exit(1)
		}

		topLevel, _ := c.gitCtx.TopLevel(dir, true)
		//		charsToChop := len(dir) - len(topLevel)
		rootImportPath := importPath[:len(importPath)+len(topLevel)-len(dir)]

		c.doneDirs[topLevel] = empty{}

		remoteOriginUrl, _ := c.gitCtx.RemoteOriginUrl(dir, true)
		SHA, _ := c.gitCtx.SHA(dir, true)

		result = append(result, PkgDep{rootImportPath, remoteOriginUrl, SHA})
	}
	return result
}

//pkg string should be a space delimited list of packages including all subfolders
//typically ./...
func (c *Context) Snapshot(workingDir, pkgString string) DepsFile {
	list, _ := c.goCtx.List(workingDir, pkgString)

	allTestImports := set{}
	regDeps := set{}

	for _, e := range list {

		if testImportsInt, ok := e["TestImports"]; ok {
			testImports := testImportsInt.([]interface{})
			for _, testImport := range testImports {
				allTestImports[testImport.(string)] = empty{}
			}
		}

		if depsInt, ok := e["Deps"]; ok {
			deps := depsInt.([]interface{})
			for _, dep := range deps {
				regDeps[dep.(string)] = empty{}
			}
		}
	}

	allTestImportList := []string{}
	for i, _ := range allTestImports {
		allTestImportList = append(allTestImportList, i)
	}

	testDeps := set{}
	if len(allTestImportList) > 0 {
		testList, _ := c.goCtx.List(workingDir, strings.Join(allTestImportList, " "))
		for _, e := range testList {
			if depsInt, ok := e["Deps"]; ok {
				deps := depsInt.([]interface{})
				for _, dep := range deps {
					if _, isRegDep := regDeps[dep.(string)]; isRegDep {
						continue
					}
					testDeps[dep.(string)] = empty{}
				}
			}
		}
	}

	depsScan := c.scanDeps(list, workingDir, regDeps)
	testDepsScan := c.scanDeps(list, workingDir, testDeps)

	return DepsFile{Deps: depsScan, TestDeps: testDepsScan}

}

func (c *Context) reproduceDep(pkgDep PkgDep) {
	dir := filepath.Join(c.goPath[0], "src", pkgDep.ImportPath)
	if dsutil.CheckPath(dir) {
		c.output.Error("Package '%s' can't be cloned to '%s', path already exists.", pkgDep.ImportPath, dir)
		os.Exit(1)
	}

	c.gitCtx.Clone(dir, pkgDep.GitRemote, true)
	sha, _ := c.gitCtx.SHA(dir, true)
	if sha != pkgDep.SHA {
		c.gitCtx.Checkout(dir, pkgDep.SHA, true)
	}
}

func (c *Context) Reproduce(workingDir string, depsFile DepsFile, doTests bool) {
	for _, pkgDep := range depsFile.Deps {
		c.reproduceDep(pkgDep)
	}
	if doTests {
		for _, pkgDep := range depsFile.TestDeps {
			c.reproduceDep(pkgDep)
		}
	}
}

func ReadJson(filename string) (DepsFile, error) {
	var result DepsFile

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return result, err
	}

	json.Unmarshal(file, &result)
	return result, nil
}

func WriteJson(filename string, depsFile DepsFile) error {
	jsonOutput, err := json.MarshalIndent(&depsFile, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, jsonOutput, 0644)
}
