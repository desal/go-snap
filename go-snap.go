package main

import (
	"os"
	"strings"

	"github.com/desal/cmd"
	"github.com/desal/go-snap/snapshot"
	"github.com/desal/gocmd"
	"github.com/desal/richtext"
	"github.com/jawher/mow.cli"
)

func setupContext(output cmd.Output, verbose bool) *snapshot.Context {
	goPath := gocmd.FromEnv(output)
	return snapshot.NewContext(output, goPath)
}

func main() {
	app := cli.App("go-snap", "Go dependency snapshot management")
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
			output := cmd.NewStdOutput(*verbose, richtext.Ansi())

			ctx := setupContext(output, *verbose)
			depsFile := ctx.Snapshot(".", strings.Join(*pkgs, " "))
			err := snapshot.WriteJson(*filename, depsFile)
			if err != nil {
				output.Error("Could not write snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}
		}
	})

	app.Command("reproduce", "Reproduces environment from file", func(c *cli.Cmd) {

		var (
			skipTests = c.BoolOpt("t notests", false, "Skip dependencies used exclusively for tests")
		)

		c.Action = func() {
			output := cmd.NewStdOutput(*verbose, richtext.Ansi())
			ctx := setupContext(output, *verbose)

			depsFile, err := snapshot.ReadJson(*filename)
			if err != nil {
				output.Error("Could not read snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}

			ctx.Reproduce(".", depsFile, !*skipTests)
		}
	})

	app.Run(os.Args)
}
