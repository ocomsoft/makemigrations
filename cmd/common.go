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

	"github.com/ocomsoft/makemigrations/internal/struct2schema"
)

// ExecuteStruct2Schema handles the complete struct-to-schema conversion process
func ExecuteStruct2Schema(inputDir, outputFile, configFile, targetDB string, dryRun, verbose bool) error {
	if verbose {
		fmt.Println("struct2schema - Go struct to YAML schema converter")
		fmt.Println("=============================================")
	}

	// Initialize the struct2schema processor
	processor, err := struct2schema.NewProcessor(struct2schema.ProcessorConfig{
		InputDir:   inputDir,
		OutputFile: outputFile,
		ConfigFile: configFile,
		TargetDB:   targetDB,
		DryRun:     dryRun,
		Verbose:    verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize processor: %w", err)
	}

	// Process the structs
	if err := processor.Process(); err != nil {
		return fmt.Errorf("failed to process structs: %w", err)
	}

	if dryRun {
		fmt.Println("\nDry run completed successfully - no files were modified")
	} else {
		if verbose {
			fmt.Printf("\nSchema file written to: %s\n", outputFile)
		}
		fmt.Println("struct2schema completed successfully")
	}

	return nil
}

