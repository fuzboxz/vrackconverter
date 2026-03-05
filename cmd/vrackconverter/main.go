package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"vrackconverter/internal/converter"
)

var (
	Version     = "dev"
	showHelp    bool
	showVersion bool
	outputPath  string
	overwrite   bool
	quiet       bool
	metamodule  bool
)

func init() {
	flag.BoolVar(&showHelp, "h", false, "Show help")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showVersion, "V", false, "Show version")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.StringVar(&outputPath, "o", "", "Output file/directory")
	flag.StringVar(&outputPath, "output", "", "Output file/directory")
	flag.BoolVar(&overwrite, "overwrite", false, "Overwrite input file in place")
	flag.BoolVar(&quiet, "q", false, "Suppress non-error output")
	flag.BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	flag.BoolVar(&metamodule, "m", false, "Add 4ms MetaModule (HubMedium) to patch")
	flag.BoolVar(&metamodule, "metamodule", false, "Add 4ms MetaModule (HubMedium) to patch")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `vrackconverter - Convert VCV Rack v0.6 compatible patches (including MiRack) to v2.0 format

Usage:
  vrackconverter <input> -o <output>     # Convert to new file
  vrackconverter <input> --overwrite     # Overwrite input file in place
  vrackconverter <input.mrk>             # Auto-create .vcv (never modifies .mrk)
  vrackconverter <dir-with-mrk>         # Auto-creates .vcv in same directory
  vrackconverter <dir> -o <output>       # Batch convert directory

Arguments:
  <input>    Input .vcv or .mrk file, or directory

Flags:
  -o, --output <path>    Output file/directory (if not specified, requires --overwrite)
      --overwrite        Overwrite input file in place
  -m, --metamodule       Add 4ms MetaModule (HubMedium) to patch
  -q, --quiet            Suppress non-error output
  -V, --version          Show version
  -h, --help             Show this help

Examples:
  vrackconverter old-patch.vcv -o new-patch.vcv
  vrackconverter old-patch.vcv --overwrite
  vrackconverter my-patch.mrk                      # Creates my-patch.vcv
  vrackconverter my-patch.mrk -o converted.vcv
  vrackconverter ./mrk-patches/                    # Creates .vcv alongside .mrk
  vrackconverter ./patches/ -o ./converted/        # Batch with output dir
`)
}

func isMrkFile(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".mrk"
}

// directoryContainsMrkFiles detects if directory contains .mrk files but not .vcv files.
// This determines whether to auto-generate output for .mrk directories.
func directoryContainsMrkFiles(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	hasMrk, hasVcv := false, false
	for _, e := range entries {
		if e.IsDir() {
			// Check for .mrk bundle directories
			if strings.ToLower(filepath.Ext(e.Name())) == ".mrk" {
				hasMrk = true
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mrk" {
			hasMrk = true
		} else if ext == ".vcv" {
			hasVcv = true
		}
	}
	return hasMrk && !hasVcv
}

func main() {
	flag.Usage = printUsage

	// Parse flags, allowing them anywhere in the command line
	args := os.Args[1:]

	// Manually parse flags that can appear anywhere
	// Go's flag package stops at first positional arg, so we handle these ourselves
	var cleanedArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-o", "--output":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++ // skip the value
			}
		case "--overwrite":
			overwrite = true
		case "-q", "--quiet":
			quiet = true
		case "-m", "--metamodule":
			metamodule = true
		default:
			// Keep positional args and unknown flags for flag.Parse()
			cleanedArgs = append(cleanedArgs, arg)
		}
	}

	// Now parse remaining flags (for -h, -V, etc.)
	flag.CommandLine.Parse(cleanedArgs)

	if showVersion {
		fmt.Printf("vrackconverter version %s\n", Version)
		os.Exit(0)
	}

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// Remaining args after flag parsing are positional
	positionalArgs := flag.Args()
	if len(positionalArgs) == 0 {
		fmt.Fprintln(os.Stderr, "error: input path required")
		printUsage()
		os.Exit(1)
	}

	inputPath := positionalArgs[0]

	opts := converter.Options{
		Overwrite:  overwrite,
		Quiet:      quiet,
		MetaModule: metamodule,
	}

	// Check for .mrk bundles BEFORE directory check, since .mrk bundles are directories
	// but should be treated as single files for conversion
	if isMrkFile(inputPath) {
		// .mrk is a directory bundle - look for patch.vcv inside
		mrkPath := inputPath
		actualInput := filepath.Join(inputPath, "patch.vcv")
		if _, err := os.Stat(actualInput); err != nil {
			fmt.Fprintf(os.Stderr, "error: .mrk bundle must contain patch.vcv: %v\n", err)
			os.Exit(1)
		}
		inputPath = actualInput

		if outputPath == "" {
			// Auto-generate output path: replace .mrk with .vcv
			baseName := mrkPath[:len(mrkPath)-len(filepath.Ext(mrkPath))]
			outputPath = baseName + ".vcv"
			if !quiet {
				fmt.Fprintf(os.Stderr, "info: .mrk input detected, creating %s\n", outputPath)
			}
		}
		// Note: .mrk files themselves are never modified since outputPath != inputPath
		// The --overwrite flag controls whether the auto-generated .vcv can be overwritten
		convertFile(inputPath, outputPath, opts)
		return
	}

	// Check if input is a directory (for batch conversion of .vcv or .mrk files)
	if converter.IsDirectory(inputPath) {
		// Directory handling
		if outputPath == "" && !overwrite {
			// Check if directory contains .mrk files (but not .vcv files)
			if directoryContainsMrkFiles(inputPath) {
				// Auto-generate output: convert in place (creates .vcv next to .mrk)
				outputPath = inputPath
				if !quiet {
					fmt.Fprintf(os.Stderr, "info: .mrk directory detected, creating .vcv files in same directory\n")
				}
			} else {
				// .vcv directory requires explicit output
				fmt.Fprintln(os.Stderr, "error: must specify -o <output> or --overwrite for .vcv directories")
				printUsage()
				os.Exit(1)
			}
		}
		convertDirectory(inputPath, outputPath, opts)
		return
	}

	// For non-.mrk files
	if outputPath == "" && !overwrite {
		fmt.Fprintln(os.Stderr, "error: must specify -o <output> or --overwrite")
		fmt.Fprintln(os.Stderr, "  (to convert in place and overwrite the input file)")
		printUsage()
		os.Exit(1)
	}
	if outputPath == "" && overwrite {
		// In-place conversion: output = input
		outputPath = inputPath
	}

	// Single file conversion
	convertFile(inputPath, outputPath, opts)
}

func convertFile(inputPath, outputPath string, opts converter.Options) {
	if !opts.Quiet {
		if inputPath == outputPath {
			fmt.Printf("Converting: %s (in place)\n", inputPath)
		} else {
			fmt.Printf("Converting: %s -> %s\n", inputPath, outputPath)
		}
	}

	result := converter.ConvertFile(inputPath, outputPath, opts)
	if result.Skipped {
		// File is already v2 format - informational, not an error
		fmt.Fprintf(os.Stderr, "info: file is already in VCV Rack v2 format (no conversion needed)\n")
		os.Exit(0)
	}
	if !result.Success {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Error)
		os.Exit(1)
	}

	if !opts.Quiet {
		if len(result.Issues) > 0 {
			fmt.Printf("  Warnings:\n")
			for _, issue := range result.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}
		fmt.Println("  Done!")
	}
}

func convertDirectory(inputDir, outputDir string, opts converter.Options) {
	if !opts.Quiet {
		fmt.Printf("Converting directory: %s -> %s\n", inputDir, outputDir)
	}

	results := converter.ConvertDirectory(inputDir, outputDir, opts)

	successCount := 0
	skipCount := 0
	failCount := 0

	for _, result := range results {
		relPath, _ := filepath.Rel(inputDir, result.InputPath)
		if result.Skipped {
			skipCount++
			if !opts.Quiet {
				fmt.Printf("  ⊘ %s (already v2)\n", relPath)
			}
		} else if result.Success {
			successCount++
			if !opts.Quiet {
				fmt.Printf("  ✓ %s\n", relPath)
				if len(result.Issues) > 0 {
					for _, issue := range result.Issues {
						fmt.Printf("    - %s\n", issue)
					}
				}
			}
		} else {
			failCount++
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", relPath, result.Error)
		}
	}

	if !opts.Quiet {
		if skipCount > 0 {
			fmt.Printf("\nComplete: %d succeeded, %d skipped, %d failed\n", successCount, skipCount, failCount)
		} else {
			fmt.Printf("\nComplete: %d succeeded, %d failed\n", successCount, failCount)
		}
	}

	if failCount > 0 {
		os.Exit(1)
	}
}
