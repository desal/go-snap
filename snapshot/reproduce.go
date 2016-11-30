package snapshot

import (
	"path/filepath"

	"github.com/desal/dsutil"
	"github.com/desal/git"
)

type AlreadyExists int

const (
	AlreadyExists_Fail AlreadyExists = iota
	AlreadyExists_Force
	AlreadyExists_Continue
	AlreadyExists_Check
)

func (c *Context) reproduceDep(pkgDep PkgDep, alreadyExists AlreadyExists) error {
	dir := filepath.Join(c.goPath[0], "src", pkgDep.ImportPath)
	var sha string
	var err error

	if !dsutil.CheckPath(dir) {
		err := c.reproduceGitCtx.Clone(dir, pkgDep.GitRemote)
		if err != nil {
			return c.errorf("Failed to produce %s, git clone error in %s: %s.", pkgDep.GitRemote, dir, err.Error())
		}
	} else if alreadyExists == AlreadyExists_Fail {
		return c.errorf("Failed to reproduce %s, %s already exists.", pkgDep.GitRemote, dir)
	} else if alreadyExists == AlreadyExists_Force {
		if isGit := c.reproduceGitCtx.IsGit(dir); !isGit {
			return c.errorf("Falied to reproduce %s, %s is not a git repo.", pkgDep.GitRemote, dir)
		} else if gitStatus, err := c.reproduceGitCtx.Status(dir); err != nil {
			return c.errorf("Failed to reproduce %s, could not get git status for %s: %s.", pkgDep.GitRemote, dir, err.Error())
		} else if gitStatus != git.Clean {
			return c.errorf("Failed to reproduce %s, git status for %s is %s.", pkgDep.GitRemote, dir, gitStatus.String())
		} else if err = c.reproduceGitCtx.Checkout(dir, "master"); err != nil {
			return c.errorf("Failed to checkout master, git pull error in %s: %s.", dir, err.Error())
		} else if err = c.reproduceGitCtx.Pull(dir); err != nil {
			return c.errorf("Failed to reproduce %s, git pull error in %s: %s.", pkgDep.GitRemote, dir, err.Error())
		}
	} else if alreadyExists == AlreadyExists_Continue {
		goto ok
	} else if alreadyExists == AlreadyExists_Check {
		if isGit := c.reproduceGitCtx.IsGit(dir); !isGit {
			return c.errorf("Falied to check %s, %s is not a git repo.", pkgDep.GitRemote, dir)
		} else if gitStatus, err := c.reproduceGitCtx.Status(dir); err != nil {
			return c.errorf("Failed to check %s, could not get git status for %s: %s.", pkgDep.GitRemote, dir, err.Error())
		} else if gitStatus != git.Clean {
			return c.errorf("Failed to check %s, git status for %s is %s.", pkgDep.GitRemote, dir, gitStatus.String())
		} else if sha, err := c.reproduceGitCtx.SHA(dir); err != nil {
			return c.errorf("Failed to check %s, could not get git sha for %s: %s.", pkgDep.GitRemote, dir, err.Error())
		} else if sha != pkgDep.SHA {
			return c.errorf("Check %s failed. Expected SHA %s, Have %s.", pkgDep.GitRemote, pkgDep.SHA, sha)
		} else {
			goto ok
		}
	}

	sha, err = c.reproduceGitCtx.SHA(dir)
	if err != nil {
		return c.errorf("Failed to reproduce %s, git error getting current SHA in %s: %s.", pkgDep.GitRemote, dir, err.Error())
	} else if sha == pkgDep.SHA {

	} else if err := c.reproduceGitCtx.Checkout(dir, pkgDep.SHA); err != nil {
		return c.errorf("Failed to reproduce %s, git error in checkout in %s: %s.", pkgDep.GitRemote, dir, err.Error())
	}
ok:
	c.verbosef("%s", pkgDep.ImportPath)

	return nil

}

func (c *Context) Reproduce(workingDir string, depsFile DepsFile, doTests bool, alreadyExists AlreadyExists) error {
	for _, pkgDep := range depsFile.Deps {
		err := c.reproduceDep(pkgDep, alreadyExists)
		if err != nil {
			return err
		}
	}
	if doTests {
		for _, pkgDep := range depsFile.TestDeps {
			err := c.reproduceDep(pkgDep, alreadyExists)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
