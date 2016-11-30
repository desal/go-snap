package snapshot_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/desal/cmd"
	"github.com/desal/dsutil"
	"github.com/desal/git"
	"github.com/desal/go-snap/snapshot"
	"github.com/desal/richtext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type goCtx struct {
	bareDir string
	gopath  string
	bareCtx *cmd.Context
	goCtx   *cmd.Context
}

func SetupRepos(t *testing.T) *goCtx {
	m := &goCtx{}
	var err error
	m.gopath, err = ioutil.TempDir("", "snapshot_test_gopath")
	require.Nil(t, err)

	m.bareDir, err = ioutil.TempDir("", "snapshot_test_bare")
	require.Nil(t, err)

	m.bareCtx = cmd.New(m.bareDir, richtext.Test(t), cmd.Strict, cmd.Warn)
	m.goCtx = cmd.New(m.gopath, richtext.Test(t), cmd.Strict, cmd.Warn)
	m.goCtx.Execf("mkdir src")
	os.Setenv("GOPATH", m.gopath)

	return m
}

func (m *goCtx) Close() {
	os.RemoveAll(m.bareDir)
	os.RemoveAll(m.gopath)
}

func (m *goCtx) AddRepo(repoName string) {
	m.bareCtx.Execf("mkdir %s; cd %s; git --bare init", repoName, repoName)
	m.goCtx.Execf("cd src; git clone %s %s", dsutil.PosixPath(m.bareDir)+"/"+repoName, repoName)
	m.goCtx.Execf("cd src/%s; touch init; git add -A; git commit -m init; git push", repoName)
}

func TestSnapshotSimple(t *testing.T) {
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
		git push`)
	m.goCtx.Execf(`
		cd src/deptwo; 
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git push`)
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
	buf := &bytes.Buffer{}
	ctx := snapshot.New(richtext.Debug(buf), []string{m.gopath}, snapshot.Verbose)
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	require.Nil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")

	assert.Equal(t,
		fmt.Sprintf("{["+
			"{depone %s/depone %s [] <nil>} "+
			"{deptwo %s/deptwo %s [] <nil>}"+
			"] []}", dsutil.PosixPath(m.bareDir), sha1, dsutil.PosixPath(m.bareDir), sha2),
		fmt.Sprintf("%v", depsFile))

	assert.Equal(t, "depone\ndeptwo\n", buf.String())
}

func TestSnapshotTests(t *testing.T) {
	m := SetupRepos(t)
	//	defer m.Close()

	m.AddRepo("depone")
	m.AddRepo("deptwo")
	m.AddRepo("mainpkg")

	m.goCtx.Execf(`
		cd src/depone; 
		echo 'package depone\n\nconst One = 12' > depone.go;
		git add -A;
		git commit -m "gocode";
		git push`)
	m.goCtx.Execf(`
		cd src/deptwo; 
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git push`)
	m.goCtx.Execf(`
		cd src/mainpkg; 
		echo '%s' > main.go;
		echo '%s' > main_test.go;
		git add -A;
		git commit -m "gocode";
		git push`, `package main
	
import ( 
	"fmt"
)

func main() { fmt.Println(0) }
`, `package main
	
import ( 
	"depone"
	"deptwo"
	"testing"
)

func TestA(t *testing.T) { t.Log(depone.One * depone.Two) }
`)

	buf := &bytes.Buffer{}
	ctx := snapshot.New(richtext.Debug(buf), []string{m.gopath}, snapshot.Verbose)
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	require.Nil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")

	assert.Equal(t,
		fmt.Sprintf("{[] ["+
			"{depone %s/depone %s [] <nil>} "+
			"{deptwo %s/deptwo %s [] <nil>}"+
			"]}", dsutil.PosixPath(m.bareDir), sha1, dsutil.PosixPath(m.bareDir), sha2),
		fmt.Sprintf("%v", depsFile))

	assert.Equal(t, "depone\ndeptwo\n", buf.String())
}

func TestSnapshotTestX(t *testing.T) {
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
		git push`)
	m.goCtx.Execf(`
		cd src/deptwo; 
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git push`)
	m.goCtx.Execf(`
		cd src/mainpkg; 
		echo '%s' > main.go;
		echo '%s' > main_test.go;
		git add -A;
		git commit -m "gocode";
		git push`, `package main
	
import ( 
	"fmt"
)

func main() { fmt.Println(0) }
`, `package main_test
	
import ( 
	"depone"
	"deptwo"
	"testing"
)

func TestA(t *testing.T) { t.Log(depone.One * depone.Two) }
`)

	buf := &bytes.Buffer{}
	ctx := snapshot.New(richtext.Debug(buf), []string{m.gopath}, snapshot.Verbose)
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	require.Nil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")

	assert.Equal(t,
		fmt.Sprintf("{[] ["+
			"{depone %s/depone %s [] <nil>} "+
			"{deptwo %s/deptwo %s [] <nil>}"+
			"]}", dsutil.PosixPath(m.bareDir), sha1, dsutil.PosixPath(m.bareDir), sha2),
		fmt.Sprintf("%v", depsFile))

	assert.Equal(t, "depone\ndeptwo\n", buf.String())
}

func TestSnapshotTags(t *testing.T) {
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
		git tag v1.0;
		git push`)
	m.goCtx.Execf(`
		cd src/deptwo; 
		echo 'package deptwo\n\nconst Two = 3' > deptwo.go;
		git add -A;
		git commit -m "gocode";
		git tag v1.0;
		git tag vAwesome;
		git push`)
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
	buf := &bytes.Buffer{}
	ctx := snapshot.New(richtext.Debug(buf), []string{m.gopath}, snapshot.Verbose)
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	require.Nil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")

	assert.Equal(t,
		fmt.Sprintf("{["+
			"{depone %s/depone %s [v1.0] <nil>} "+
			"{deptwo %s/deptwo %s [v1.0 vAwesome] <nil>}"+
			"] []}", dsutil.PosixPath(m.bareDir), sha1, dsutil.PosixPath(m.bareDir), sha2),
		fmt.Sprintf("%v", depsFile))

	assert.Equal(t, "depone\ndeptwo\n", buf.String())
}

func TestSnapshotDirty(t *testing.T) {
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
		git push;
		echo 'package depone\n\nconst One = 24' > depone.go;`)

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
		git push;
		echo 'package depthree\n\nconst Three = -2' > depthree.go;`)

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

	buf := &bytes.Buffer{}
	ctx := snapshot.New(richtext.Debug(buf), []string{m.gopath}, snapshot.Verbose)
	depsFile, err := ctx.Snapshot(m.gopath, "mainpkg", []string{""})
	assert.NotNil(t, err)

	gitCtx := git.New(richtext.Test(t), git.MustPanic)
	sha1, _ := gitCtx.SHA(m.bareDir + "/depone")
	sha2, _ := gitCtx.SHA(m.bareDir + "/deptwo")
	sha3, _ := gitCtx.SHA(m.bareDir + "/depthree")

	dirPrefix := dsutil.PosixPath(m.bareDir) + "/"

	assert.Equal(t, "depone", depsFile.Deps[0].ImportPath)
	assert.Equal(t, dirPrefix+"depone", depsFile.Deps[0].GitRemote)
	assert.Equal(t, 0, len(depsFile.Deps[0].Tags))
	assert.Equal(t, sha1, depsFile.Deps[0].SHA)
	assert.NotNil(t, depsFile.Deps[0].Error)

	assert.Equal(t, "deptwo", depsFile.Deps[2].ImportPath)
	assert.Equal(t, dirPrefix+"deptwo", depsFile.Deps[2].GitRemote)
	assert.Equal(t, 0, len(depsFile.Deps[2].Tags))
	assert.Equal(t, sha2, depsFile.Deps[2].SHA)
	assert.Nil(t, depsFile.Deps[2].Error)

	assert.Equal(t, "depthree", depsFile.Deps[1].ImportPath)
	assert.Equal(t, dirPrefix+"depthree", depsFile.Deps[1].GitRemote)
	assert.Equal(t, 0, len(depsFile.Deps[1].Tags))
	assert.Equal(t, sha3, depsFile.Deps[1].SHA)
	assert.NotNil(t, depsFile.Deps[1].Error)

	assert.Equal(t, fmt.Sprintf("[WARN]%s[]\n[WARN]%s[]\ndeptwo\n", depsFile.Deps[0].Error, depsFile.Deps[1].Error), buf.String())
}
