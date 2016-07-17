package main

import (
	"os"
	"strings"

	"github.com/desal/go-snap/snapshot"
	"github.com/desal/gocmd"
	"github.com/desal/richtext"
	"github.com/jawher/mow.cli"
)

func setupContext(format richtext.Format, verbose bool) *snapshot.Context {
	goPath, err := gocmd.EnvGoPath()
	if err != nil {
		format.ErrorLine("Failed to get GOPATH: %s", err.Error())
		os.Exit(1)
	}
	var flags []snapshot.Flag
	if verbose {
		flags = append(flags, snapshot.Verbose)
	}
	return snapshot.New(format, goPath, flags...)
}

func main() {
	app := cli.App("go-snap", "Go dependency snapshot management")
	format := richtext.Ansi()

	var (
		filename = app.StringOpt("f filename", "snapshot.json", "filename to save snapshot to")
		verbose  = app.BoolOpt("v verbose", false, "Verbose output")
	)
	app.Command("snapshot", "Takes a snapshot of all currently used dependencies", func(c *cli.Cmd) {
		c.Spec = "PKG..."

		var (
			pkgs = c.StringsArg("PKG", nil, "Packages to snapshot")
		)

		c.Action = func() {
			ctx := setupContext(format, *verbose)
			depsFile, err := ctx.Snapshot(".", strings.Join(*pkgs, " "))

			if err != nil {
				format.ErrorLine("%s", err.Error())
			}

			err = snapshot.WriteJson(*filename, depsFile)
			if err != nil {
				format.ErrorLine("Could not write snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}
		}
	})

	app.Command("reproduce", "Reproduces environment from file", func(c *cli.Cmd) {

		var (
			skipTests = c.BoolOpt("t notests", false, "Skip dependencies used exclusively for tests")
			force     = c.BoolOpt("f force", false, "Force dependencies to version, even if they exist")
		)

		c.Action = func() {
			ctx := setupContext(format, *verbose)

			depsFile, err := snapshot.ReadJson(*filename)
			if err != nil {
				format.ErrorLine("Could not read snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}

			ctx.Reproduce(".", depsFile, !*skipTests, *force)
		}
	})

	app.Run(os.Args)
}
