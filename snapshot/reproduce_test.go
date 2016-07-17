package snapshot_test

import (
	"testing"

	"github.com/desal/dsutil"
	"github.com/desal/git"
	"github.com/desal/go-snap/snapshot"
	"github.com/desal/richtext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReproduceSimple(t *testing.T) {
	m := SetupRepos(t)
	defer m.Close()

	m.AddRepo("depone")
	m.AddRepo("deptwo")
	m.AddRepo("mainpkg")

	m.goCtx.Execf(`
		cd src/depone; 
		echo 'package depone\n\nconst One = 12' > depone.go;
		git add -A;
		git commit -m "gocode";
		git push;
		cd ..;
		rm -rf depone`)
	m.goCtx.Execf(`
		cd src/deptwo; 
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git push
		cd ..;
		rm -rf deptwo`)
	m.goCtx.Execf(`
		cd src/mainpkg; 
		echo '%s' > main.go;
		git add -A;
		git commit -m "gocode";
		git push`, `package main
	
import ( 
	"fmt"
	"depone"
	"deptwo"
)

func main() { fmt.Println(depone.One * deptwo.Two) }
`)
	files, _, _ := m.goCtx.Execf(`find . -not -path "*/.git*"`)
	assert.Equal(t, `.
./src
./src/mainpkg
./src/mainpkg/init
./src/mainpkg/main.go
`, files)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")

	ctx := snapshot.New(richtext.Test(t), []string{m.gopath})
	depsFile := snapshot.DepsFile{
		Deps: []snapshot.PkgDep{
			snapshot.PkgDep{"depone", dsutil.PosixPath(m.bareDir) + "/depone", sha1, []string{}},
			snapshot.PkgDep{"deptwo", dsutil.PosixPath(m.bareDir) + "/deptwo", sha2, []string{}},
		},
	}

	err := ctx.Reproduce(m.gopath, depsFile, false, false)
	require.Nil(t, err)
	files, _, _ = m.goCtx.Execf(`find . -not -path "*/.git*"`)
	assert.Equal(t, `.
./src
./src/depone
./src/depone/depone.go
./src/depone/init
./src/deptwo
./src/deptwo/deptwo.go
./src/deptwo/init
./src/mainpkg
./src/mainpkg/init
./src/mainpkg/main.go
`, files)
}
