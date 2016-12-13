package snapshot_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/desal/dsutil"
	"github.com/desal/git"
	"github.com/desal/go-snap/snapshot"
	"github.com/desal/richtext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//TODO More tests t
//Diff test, 3 deps, 1 dirty, 1 new commit (with new tag), just check output

func TestCompare(t *testing.T) {

	m := SetupRepos(t)
	defer m.Close()

	m.AddRepo("depone")
	m.AddRepo("deptwo")
	m.AddRepo("depthree")
	m.AddRepo("mainpkg")

	m.goCtx.Execf(`
		cd src/depone;
		echo 'package depone\n\nconst One = 12' > depone.go;
		git add -A;
		git commit -m "gocode";
		git tag v1.0;
		git push;`)

	m.goCtx.Execf(`
		cd src/deptwo;
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git push`)

	m.goCtx.Execf(`
		cd src/depthree;
		echo 'package depthree\n\nconst Three = -1' > depthree.go;
		git add -A;
		git commit -m "gocode";
		git push;`)

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
		"depthree"
	)

	func main() { fmt.Println(depone.One * deptwo.Two * depthree.Three) }
	`)

	ctx := snapshot.New(richtext.Test(t), []string{m.gopath})
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	assert.Nil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")
	sha3, _ := gitCtx.SHA(m.bareDir + "/depthree")

	dirPrefix := dsutil.PosixPath(m.bareDir) + "/"

	assert.Equal(t, "depone", depsFile.Deps[0].ImportPath)
	assert.Equal(t, dirPrefix+"depone", dsutil.PosixPath(depsFile.Deps[0].GitRemote))
	require.Equal(t, 1, len(depsFile.Deps[0].Tags))
	assert.Equal(t, "v1.0", depsFile.Deps[0].Tags[0])
	assert.Equal(t, sha1, depsFile.Deps[0].SHA)
	assert.Nil(t, depsFile.Deps[0].Error)

	assert.Equal(t, "deptwo", depsFile.Deps[2].ImportPath)
	assert.Equal(t, dirPrefix+"deptwo", dsutil.PosixPath(depsFile.Deps[2].GitRemote))
	assert.Equal(t, 0, len(depsFile.Deps[2].Tags))
	assert.Equal(t, sha2, depsFile.Deps[2].SHA)
	assert.Nil(t, depsFile.Deps[2].Error)

	assert.Equal(t, "depthree", depsFile.Deps[1].ImportPath)
	assert.Equal(t, dirPrefix+"depthree", dsutil.PosixPath(depsFile.Deps[1].GitRemote))
	assert.Equal(t, 0, len(depsFile.Deps[1].Tags))
	assert.Equal(t, sha3, depsFile.Deps[1].SHA)
	assert.Nil(t, depsFile.Deps[1].Error)

	sha1before, _ := gitCtx.SHA(m.bareDir + "/depone")

	m.goCtx.Execf(`
		cd src/depone;
		echo 'package depone\n\nconst One = 24' > depone.go;
		git add -A;
		git commit -m "changes";
		git tag v2.0;
		git push;`)

	m.goCtx.Execf(`
		cd src/depthree;
		echo 'package depthree\n\nconst Three = -2' > depthree.go`)

	sha1after, _ := gitCtx.SHA(m.bareDir + "/depone")

	buf := &bytes.Buffer{}
	compareCtx := snapshot.New(richtext.Debug(buf), []string{m.gopath})
	stripTime(&depsFile)
	result, ok := compareCtx.Compare(m.gopath, "mainpkg", []string{""}, depsFile, true)
	assert.False(t, ok)

	assert.NotEqual(t, "", result[1].Message)
	assert.Contains(t, result[1].Message, "Import depthree")
	assert.Contains(t, result[1].Message, "has git status Uncommitted")

	assert.Equal(t, fmt.Sprintf(`[Red,None,[Bold]][FAIL][] depone   (expected) %s [v1.0] vs (actual) %s [v2.0]
[Red,None,[Bold]][FAIL][] depthree %s
[Green,None,[Bold]][ OK ][] deptwo   
`, sha1before[0:6], sha1after[0:6], result[1].Message), buf.String())
}

//TODO more output tests like this
