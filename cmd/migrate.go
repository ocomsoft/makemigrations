/*
MIT License

# Copyright (c) 2025 OcomSoft

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/config"
)

// migrateCmd compiles the migrations module and runs it with the given args.
// DisableFlagParsing passes all arguments — including any flags intended for
// the binary — through unchanged. --verbose / -v and --dont-upgrade are
// intercepted locally and stripped before forwarding.
var migrateCmd = &cobra.Command{
	Use:   "migrate [args...]",
	Short: "Build and run the compiled migrations binary",
	Long: `Build the compiled migrations binary for the configured migrations directory
and run it with the provided arguments. All arguments are forwarded to the
binary unchanged, so every subcommand the binary supports is available:

  makemigrations migrate up
  makemigrations migrate up --to 0005_add_index
  makemigrations migrate down --steps 2
  makemigrations migrate status
  makemigrations migrate showsql
  makemigrations migrate fake 0001_initial
  makemigrations migrate dag

Pass --verbose (or -v) to see build output:

  makemigrations migrate --verbose up

By default the command upgrades the migrations go.mod to match the running CLI
version before building. Pass --dont-upgrade to skip this step (useful in CI
environments where go.mod must not be modified at runtime):

  makemigrations migrate --dont-upgrade up`,
	DisableFlagParsing: true,
	SilenceErrors:      true,
	RunE: func(_ *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault(configFile)

		// Intercept --verbose / -v and --dont-upgrade so they control build
		// behaviour only and are not forwarded to the migrations binary.
		verbose := false
		upgrade := true
		var binaryArgs []string
		for _, a := range args {
			switch a {
			case "--verbose", "-v":
				verbose = true
			case "--dont-upgrade":
				upgrade = false
			default:
				binaryArgs = append(binaryArgs, a)
			}
		}

		return ExecuteMigrate(cfg.Migration.Directory, binaryArgs, verbose, upgrade)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

// ExecuteMigrate builds the migrations binary for migrationsDir and runs it
// with the provided args. stdin, stdout and stderr are inherited so interactive
// prompts and coloured output from the binary work correctly.
//
// When upgrade is true the migrations go.mod is brought up to the same
// makemigrations version as the running CLI before building. Pass false to
// skip this step (--dont-upgrade flag).
func ExecuteMigrate(migrationsDir string, args []string, verbose bool, upgrade bool) error {
	binPath, cleanup, err := buildMigrationsBinary(migrationsDir, verbose, upgrade)
	if err != nil {
		return fmt.Errorf("building migrations binary: %w", err)
	}
	defer cleanup()

	cmd := exec.Command(binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		// The binary has already written its error to stderr; propagate the
		// exit code without adding a second error message.
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return runErr
	}
	return nil
}
