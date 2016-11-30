package snapshot

import (
	"errors"
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

//returns nil for not a dependency
func (c *Context) scanDep(startingList stringSet, workingDir string, importPath string) *PkgDep {
	r := &PkgDep{ImportPath: importPath} //Initially create the object with the current importPath, and refine it to the root package if it's possible

	if c.goCtx.IsStdLib(importPath) {
		return nil
	}

	if _, isStartingPkg := startingList[importPath]; isStartingPkg {
		return nil
	}

	//NOTE this is the source of an annoying caveat, when using tags all
	//dependencies must have a buildable go source file when no tags are
	//supplied. i.e. 'go list [package]' shouldn't bomb out.

	list, err := c.goCtx.List(workingDir, importPath)
	if err != nil {
		r.Error = c.errorf("Failed to scan dependency %s: %s.", importPath, err.Error())
		return r
	}

	dir := list[importPath]["Dir"].(string)

	if !strings.HasSuffix(filepath.ToSlash(dir), importPath) {
		r.Error = c.errorf("Falied to scan dependency: directory %s should end in %s.", filepath.ToSlash(dir), importPath)
		return r
	}

	if c.doneRootDir(dir) {
		return nil
	}

	if !c.snapGitCtx.IsGit(dir) {
		r.Error = c.errorf("Import %s (%s) is not a git repository", importPath, dir)
		return r
	}

	status, _ := c.snapGitCtx.Status(dir)
	if status == git.NotMaster {
		c.warnf("Import %s (%s) is not origin/master", importPath, dir)
	} else if status != git.Clean {
		r.Error = c.errorf("Import %s (%s) has git status %s", importPath, dir, status.String())
	}

	topLevel, _ := c.snapGitCtx.TopLevel(dir)

	//Example
	// dir            = c:\\dev\\golang\\src\\github.com\\desal\\go-snap\\snapshot
	// topLevel       = c:\\dev\\golang\\src\\github.com\\desal\\go-snap
	// importPath     = github.com/desal/go-snap/snapshot/snapshot
	// rootImportPath = github.com/desal/go-snap/snapshot
	r.ImportPath = importPath[:len(importPath)+len(topLevel)-len(dir)]
	c.doneDirs[topLevel] = empty{}

	remoteOriginUrl, _ := c.snapGitCtx.RemoteOriginUrl(dir)
	SHA, _ := c.snapGitCtx.SHA(dir)
	tags, _ := c.snapGitCtx.Tags(dir)

	r.GitRemote = remoteOriginUrl
	r.SHA = SHA
	r.Tags = tags
	if r.Error == nil {
		c.verbosef("%s", importPath)
	}
	return r
}

//pkg string should be a space delimited list of packages including all subfolders
//typically ./...
func (c *Context) Snapshot(workingDir, pkgString string, tagsets []string) (DepsFile, error) {

	initialPackages := stringSet{}
	regDeps := stringSet{}
	testDeps := stringSet{}

	for _, tags := range tagsets {
		var goListCtx *gocmd.Context

		if c.flags.Checked(SkipVendor) {
			goListCtx = gocmd.New(c.format, c.goPath, tags, "", gocmd.SkipVendor)
		} else {
			goListCtx = gocmd.New(c.format, c.goPath, tags, "")
		}

		list, err := goListCtx.List(workingDir, pkgString)
		if err != nil {
			return DepsFile{}, c.errorf("Failed to run go list: %s", err.Error())
		}

		allTestImports := stringSet{}
		for pkg, e := range list {
			initialPackages[pkg] = empty{}
			dir := e["Dir"].(string)

			if !c.doneRootDir(dir) {
				//In case the root dir itself doesn't contain any .go files (only sub
				//packages).

				if !c.snapGitCtx.IsGit(dir) {
					return DepsFile{}, fmt.Errorf("All scanned directories must be in a git repo")
				}

				topLevel, err := c.snapGitCtx.TopLevel(dir)
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

		if len(allTestImportList) > 0 {
			testList, _ := goListCtx.List(workingDir, strings.Join(allTestImportList, " "))
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
	}

	scanDeps := func(deps stringSet) []PkgDep {
		r := []PkgDep{}
		for _, dep := range deps.Sorted() {
			if pkgDep := c.scanDep(initialPackages, workingDir, dep); pkgDep != nil {
				r = append(r, *pkgDep)
			}
		}
		return r
	}

	r := DepsFile{
		Deps:     scanDeps(regDeps),
		TestDeps: scanDeps(testDeps),
	}

	r.Sort()

	errStrings := []string{}
	appendErrs := func(pkgDeps []PkgDep) {
		for _, dep := range pkgDeps {
			if dep.Error != nil {
				errStrings = append(errStrings, dep.Error.Error())
			}
		}
	}

	appendErrs(r.Deps)
	appendErrs(r.TestDeps)
	var err error
	if len(errStrings) != 0 {
		err = errors.New(strings.Join(errStrings, ", "))
	}
	return r, err
}
