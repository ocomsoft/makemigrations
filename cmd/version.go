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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/version"
)

var (
	versionOutputFormat string
	showBuildInfo       bool
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display version information for makemigrations.

This command shows the current version, build date, git commit, and platform information.

Output formats:
- text (default): Human-readable format
- json: JSON format for scripting

Examples:
  makemigrations version
  makemigrations version --format json
  makemigrations version --build-info`,
	RunE: runVersion,
}

func runVersion(cmd *cobra.Command, args []string) error {
	switch versionOutputFormat {
	case "json":
		buildInfo := version.GetBuildInfo()
		jsonOutput, err := json.MarshalIndent(buildInfo, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal version info: %w", err)
		}
		fmt.Println(string(jsonOutput))
	default:
		if showBuildInfo {
			fmt.Println(version.GetFullVersion())
		} else {
			fmt.Println(version.GetDisplayVersion())
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().StringVarP(&versionOutputFormat, "format", "f", "text",
		"Output format (text, json)")
	versionCmd.Flags().BoolVarP(&showBuildInfo, "build-info", "b", false,
		"Show detailed build information")
}
