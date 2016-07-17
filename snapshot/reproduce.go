package snapshot

import (
	"path/filepath"

	"github.com/desal/dsutil"
	"github.com/desal/git"
)

func (c *Context) reproduceDep(pkgDep PkgDep, force bool) error {
	dir := filepath.Join(c.goPath[0], "src", pkgDep.ImportPath)
	if !dsutil.CheckPath(dir) {
		err := c.gitCtx.Clone(dir, pkgDep.GitRemote)
		if err != nil {
			return c.errorf("Failed to produce %s, git clone error in %s: %s.", pkgDep.GitRemote, dir, err.Error())
		}
	} else if force {
		if isGit := c.gitCtx.IsGit(dir); !isGit {
			return c.errorf("Falied to reproduce %s, %s is not a git repo.", pkgDep.GitRemote, dir)
		} else if gitStatus, err := c.gitCtx.Status(dir); err != nil {
			return c.errorf("Failed to reproduce %s, could not get git status for %s: %s.", pkgDep.GitRemote, dir, err.Error())
		} else if gitStatus != git.Clean {
			return c.errorf("Failed to reproduce %s, git status for %s is %s.", pkgDep.GitRemote, dir, gitStatus.String())
		} else if err = c.gitCtx.Pull(dir); err != nil {
			return c.errorf("Failed to reproduce %s, git pull error in %s: %s.", pkgDep.GitRemote, dir, err.Error())
		}
	} else {
		return c.errorf("Failed to reproduce %s, %s already exists.", pkgDep.GitRemote, dir)
	}

	sha, err := c.gitCtx.SHA(dir)
	if err != nil {
		return c.errorf("Failed to reproduce %s, git error getting current SHA in %s: %s.", pkgDep.GitRemote, dir, err.Error())
	} else if sha == pkgDep.SHA {
		//Already on correct SHA
		return nil
	}

	if err := c.gitCtx.Checkout(dir, pkgDep.SHA); err != nil {
		return c.errorf("Failed to reproduce %s, git error in checkout in %s: %s.", pkgDep.GitRemote, dir, err.Error())
	}

	return nil

}

func (c *Context) Reproduce(workingDir string, depsFile DepsFile, doTests, force bool) error {
	for _, pkgDep := range depsFile.Deps {
		err := c.reproduceDep(pkgDep, force)
		if err != nil {
			return err
		}
	}
	if doTests {
		for _, pkgDep := range depsFile.TestDeps {
			err := c.reproduceDep(pkgDep, force)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
