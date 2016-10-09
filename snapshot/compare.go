package snapshot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/desal/richtext"
)

type CompareResult int

type ComparePkg struct {
	ImportPath    string
	Message       string
	CompareResult CompareResult
}

type ComparePkgs []ComparePkg

func (a ComparePkgs) Len() int           { return len(a) }
func (a ComparePkgs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ComparePkgs) Less(i, j int) bool { return a[i].ImportPath < a[j].ImportPath }

const (
	CompareResult_Ok CompareResult = iota
	CompareResult_Warn
	CompareResult_Error
)

func (c *Context) Compare(workingDir, pkgString string, depsFile DepsFile, dotests bool) ([]ComparePkg, bool) {
	snapshot, _ := c.Snapshot(workingDir, pkgString)

	result := []ComparePkg{}
	ok := true

	comparePkgList := func(expected, actual []PkgDep) {
		depsMap := map[string]PkgDep{}
		for _, dep := range actual {
			depsMap[dep.ImportPath] = dep
		}

		for _, expectedDep := range expected {
			actualDep, hasActual := depsMap[expectedDep.ImportPath]
			if !hasActual {
				result = append(result, ComparePkg{expectedDep.ImportPath, "No longer requried", CompareResult_Warn})
				continue
			}

			if actualDep.Error != nil {
				result = append(result, ComparePkg{expectedDep.ImportPath, actualDep.Error.Error(), CompareResult_Error})
				ok = false
			} else if actualDep.SHA != expectedDep.SHA {
				actual := actualDep.SHA[0:6]
				expected := expectedDep.SHA[0:6]

				if len(actualDep.Tags) != 0 {
					actual += fmt.Sprintf(" %v", actualDep.Tags)
				}
				if len(expectedDep.Tags) != 0 {
					expected += fmt.Sprintf(" %v", expectedDep.Tags)
				}

				result = append(result, ComparePkg{expectedDep.ImportPath, fmt.Sprintf("(expected) %s vs (actual) %s", expected, actual), CompareResult_Error})
				ok = false
			} else {
				result = append(result, ComparePkg{expectedDep.ImportPath, "", CompareResult_Ok})
			}
			delete(depsMap, expectedDep.ImportPath)
		}

		//Only remaining ones should be new dependencies
		for importPath, _ := range depsMap {
			result = append(result, ComparePkg{importPath, "New dependency", CompareResult_Error})
			ok = false
		}
	}

	comparePkgList(depsFile.Deps, snapshot.Deps)
	if dotests {
		comparePkgList(depsFile.TestDeps, snapshot.TestDeps)
	}

	sort.Sort(ComparePkgs(result))

	maxLen := 0
	for _, comparePkg := range result {
		if len(comparePkg.ImportPath) > maxLen {
			maxLen = len(comparePkg.ImportPath)
		}
	}

	padding := strings.Repeat(" ", maxLen)

	green := c.format.MakePrintf(richtext.Green, richtext.None, richtext.Bold)
	orange := c.format.MakePrintf(richtext.Orange, richtext.None, richtext.Bold)
	red := c.format.MakePrintf(richtext.Red, richtext.None, richtext.Bold)

	richPrefix := map[CompareResult]func(){
		CompareResult_Ok:    func() { green("[ OK ]") },
		CompareResult_Warn:  func() { orange("[WARN]") },
		CompareResult_Error: func() { red("[FAIL]") },
	}

	for _, comparePkg := range result {
		richPrefix[comparePkg.CompareResult]()
		c.format.PrintLine(" %s %s", (comparePkg.ImportPath + padding)[0:maxLen], comparePkg.Message)
	}
	return result, ok
}