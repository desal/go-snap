package snapshot

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/desal/git"
	"github.com/desal/gocmd"
)

func (c *Context) doneRootDir(dir string) bool {
	if _, done := c.doneDirs[dir]; done {
		return true
	}

	for doneDir, _ := range c.doneDirs {
		//To not confuse pkgtwo/ as a subdirectory of pkg/
		if strings.HasPrefix(dir, doneDir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (c *Context) scanDeps(startingList map[string]map[string]interface{}, workingDir string, deps stringSet) ([]PkgDep, error) {
	result := []PkgDep{}
	for importPath, _ := range deps {
		if c.goCtx.IsStdLib(importPath) {
			continue
		}

		if _, isStartingPkg := startingList[importPath]; isStartingPkg {
			continue
		}

		list, err := c.goCtx.List(workingDir, importPath)
		if err != nil {
			return nil, c.errorf("Failed to scan dependency %s: %s.", importPath, err.Error())
		}

		dir := list[importPath]["Dir"].(string)

		if !strings.HasSuffix(filepath.ToSlash(dir), importPath) {
			return nil, c.errorf("Falied to scan dependency: directory %s should end in %s.", filepath.ToSlash(dir), importPath)
		}

		if c.doneRootDir(dir) {
			continue
		}

		if !c.gitCtx.IsGit(dir) {
			return nil, c.errorf("Import %s (%s) is not a git repository", importPath, dir)
		}

		status, _ := c.gitCtx.Status(dir)
		if status == git.NotMaster {
			c.warnf("Import %s (%s) is not origin/master", importPath, dir)
		} else if status != git.Clean {
			return nil, c.errorf("Import %s (%s) has git status %s", importPath, dir, status.String())
		}

		topLevel, _ := c.gitCtx.TopLevel(dir)

		//Example
		// dir            = c:\\dev\\golang\\src\\github.com\\desal\\go-snap\\snapshot
		// topLevel       = c:\\dev\\golang\\src\\github.com\\desal\\go-snap
		// importPath     = github.com/desal/go-snap/snapshot/snapshot
		// rootImportPath = github.com/desal/go-snap/snapshot
		rootImportPath := importPath[:len(importPath)+len(topLevel)-len(dir)]

		c.doneDirs[topLevel] = empty{}

		remoteOriginUrl, _ := c.gitCtx.RemoteOriginUrl(dir)
		SHA, _ := c.gitCtx.SHA(dir)
		tags, _ := c.gitCtx.Tags(dir)
		result = append(result, PkgDep{rootImportPath, remoteOriginUrl, SHA, tags})
	}
	return result, nil
}

//pkg string should be a space delimited list of packages including all subfolders
//typically ./...
func (c *Context) Snapshot(workingDir, pkgString string) (DepsFile, error) {
	var goListCtx *gocmd.Context

	if c.flags.Checked(SkipVendor) {
		goListCtx = gocmd.New(c.format, c.goPath)
	} else {
		goListCtx = c.goCtx
	}

	list, err := goListCtx.List(workingDir, pkgString)
	if err != nil {
		return DepsFile{}, c.errorf("Failed to run go list: %s", err.Error())
	}

	allTestImports := stringSet{}
	regDeps := stringSet{}

	for _, e := range list {
		dir := e["Dir"].(string)

		if !c.doneRootDir(dir) {
			//In case the root dir itself doesn't contain any .go files (only sub
			//packages).

			if !c.gitCtx.IsGit(dir) {
				return DepsFile{}, fmt.Errorf("All scanned directories must be in a git repo")
			}

			topLevel, err := c.gitCtx.TopLevel(dir)
			if err != nil {
				return DepsFile{}, err
			}
			c.doneDirs[topLevel] = empty{}

		}

		c.doneDirs[dir] = empty{}

		if testImportsInt, ok := e["TestImports"]; ok {
			testImports := testImportsInt.([]interface{})
			for _, testImport := range testImports {
				allTestImports[testImport.(string)] = empty{}
			}
		}

		if xTestImportsInt, ok := e["XTestImports"]; ok {
			xTestImports := xTestImportsInt.([]interface{})
			for _, testImport := range xTestImports {
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

	testDeps := stringSet{}
	if len(allTestImportList) > 0 {
		testList, _ := c.goCtx.List(workingDir, strings.Join(allTestImportList, " "))
		for _, e := range testList {
			if depsInt, ok := e["Deps"]; ok {
				deps := depsInt.([]interface{})
				for _, dep := range deps {
					allTestImportList = append(allTestImportList, dep.(string))
				}

				for _, dep := range allTestImportList {
					if _, isRegDep := regDeps[dep]; isRegDep {
						continue
					}
					testDeps[dep] = empty{}
				}
			}
		}
	}

	depsScan, err := c.scanDeps(list, workingDir, regDeps)
	if err != nil {
		return DepsFile{}, err
	}

	testDepsScan, err := c.scanDeps(list, workingDir, testDeps)
	if err != nil {
		return DepsFile{}, err
	}

	r := DepsFile{Deps: depsScan, TestDeps: testDepsScan}
	r.Sort()

	return r, nil
}
