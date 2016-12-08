package main

import (
	"os"
	"strings"

	"github.com/desal/go-snap/snapshot"
	"github.com/desal/gocmd"
	"github.com/desal/richtext"
	"github.com/jawher/mow.cli"
)

func setupContext(format richtext.Format, verbose, veryVerbose bool) *snapshot.Context {
	goPath, err := gocmd.EnvGoPath()
	if err != nil {
		format.ErrorLine("Failed to get GOPATH: %s", err.Error())
		os.Exit(1)
	}
	var flags []snapshot.Flag
	if veryVerbose {
		flags = append(flags, snapshot.Verbose, snapshot.CmdVerbose)
	} else if verbose {
		flags = append(flags, snapshot.Verbose)
	}

	return snapshot.New(format, goPath, flags...)
}

func main() {
	app := cli.App("go-snap", "Go dependency snapshot management")
	format := richtext.New()

	var (
		filename    = app.StringOpt("f filename", "snapshot.json", "filename to save snapshot to")
		verbose     = app.BoolOpt("v verbose", false, "Verbose output")
		veryVerbose = app.BoolOpt("vv veryverbose", false, "Verbose output and verbose command output")
	)
	app.Command("snapshot", "Takes a snapshot of all currently used dependencies", func(c *cli.Cmd) {
		c.Spec = "[--tags...] PKG..."
		var (
			tagSets = c.StringsOpt("tags", nil, "capture with tags (can be repeated)")
			pkgs    = c.StringsArg("PKG", nil, "Packages to snapshot")
		)

		c.Action = func() {
			if len(*tagSets) == 0 {
				*tagSets = append(*tagSets, "")
			}
			ctx := setupContext(format, *verbose, *veryVerbose)
			depsFile, err := ctx.Snapshot(".", strings.Join(*pkgs, " "), *tagSets)

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
		c.Spec = "[-t] [-f | -i | -c]"
		var (
			skipTests = c.BoolOpt("t notests", false, "Skip dependencies used exclusively for tests")
			force     = c.BoolOpt("f force", false, "Force dependencies to version, even if they exist")
			ignore    = c.BoolOpt("i ignore", false, "Continue if an existing dependency is found")
			check     = c.BoolOpt("c check", false, "If an existing dependency is found, check it against file")
		)

		c.Action = func() {
			ctx := setupContext(format, *verbose, *veryVerbose)

			depsFile, err := snapshot.ReadJson(*filename)
			if err != nil {
				format.ErrorLine("Could not read snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}

			alreadyExists := snapshot.AlreadyExists_Fail
			if *force {
				alreadyExists = snapshot.AlreadyExists_Force
			} else if *ignore {
				alreadyExists = snapshot.AlreadyExists_Continue
			} else if *check {
				alreadyExists = snapshot.AlreadyExists_Check
			}

			err = ctx.Reproduce(".", depsFile, !*skipTests, alreadyExists)
			if err != nil {
				format.ErrorLine("%s", err.Error())
				os.Exit(1)
			}
		}
	})

	app.Command("compare", "Compares snapshot.json to what's currently used to build", func(c *cli.Cmd) {
		c.Spec = "[--tags] [-t] PKG..."

		var (
			tagSets   = c.StringsOpt("tags", nil, "capture with tags (can be repeated)")
			skipTests = c.BoolOpt("t notests", false, "Skip dependencies used exclusively for tests")
			pkgs      = c.StringsArg("PKG", nil, "Packages to snapshot")
		)

		c.Action = func() {
			if tagSets == nil {
				tagSets = &[]string{""}
			}

			ctx := setupContext(format, *verbose, *veryVerbose)

			depsFile, err := snapshot.ReadJson(*filename)
			if err != nil {
				format.ErrorLine("Could not read snapshot '%s': %s", *filename, err.Error())
				os.Exit(1)
			}

			_, ok := ctx.Compare(".", strings.Join(*pkgs, " "), *tagSets, depsFile, !*skipTests)
			if !ok {
				os.Exit(1)
			}
			os.Exit(0)
		}

	})
	app.Run(os.Args)
}
