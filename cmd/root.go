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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/ocomsoft/morphic/internal/drivers"
	"github.com/ocomsoft/morphic/internal/version"
)

var (
	cfgFile    string
	configFile string // Config file path
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "morphic",
	Short: "Django-style Go migration generator",
	Long: `Generate database migrations from YAML schema files as typed Go code.

Available commands:
  init               Initialize migrations directory and create initial migration
  generate           Generate Go migration files from YAML schema changes
  migrate            Run migrations in-process via the yaegi interpreter
  diff               Show schema drift between YAML and migration state
  db-diff            Compare live database schema against migration DAG state
  current-state      Show the reconstructed schema state from existing migrations
  db-to-schema       Extract database schema to YAML
  struct-to-schema   Convert Go structs to YAML schema
  dump-sql           Dump merged YAML schema as SQL
  dump-data          Generate a migration that seeds table data using UpsertData
  schema-to-diagram  Generate diagram from YAML schema
  find-includes      Discover schema includes in Go modules
  empty              Create a blank migration with no operations`,
}

// GetRootCmd returns the root command for embedding in other applications
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Display version at startup for all commands
	fmt.Printf("%s\n", version.GetDisplayVersion())
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flag for config file
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path (default: migrations/morphic.config.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".morphic" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".morphic")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
