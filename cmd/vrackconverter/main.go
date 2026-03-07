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
	Version      = "dev"
	showHelp     bool
	showVersion  bool
	outputPath   string
	outputFormat string
	overwrite    bool
	quiet        bool
	metamodule   bool
)

func init() {
	flag.BoolVar(&showHelp, "h", false, "Show help")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showVersion, "V", false, "Show version")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.StringVar(&outputPath, "o", "", "Output file/directory")
	flag.StringVar(&outputPath, "output", "", "Output file/directory")
	flag.StringVar(&outputFormat, "format", "", "Output format: v2, v06, or mirack (overrides file extension)")
	flag.BoolVar(&overwrite, "overwrite", false, "Overwrite input file in place")
	flag.BoolVar(&quiet, "q", false, "Suppress non-error output")
	flag.BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	flag.BoolVar(&metamodule, "m", false, "Add 4ms MetaModule (HubMedium) to patch")
	flag.BoolVar(&metamodule, "metamodule", false, "Add 4ms MetaModule (HubMedium) to patch")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `vrackconverter - Convert between VCV Rack and MiRack patch formats

Usage:
  vrackconverter <input> -o <output>     # Convert to new file
  vrackconverter <input> --overwrite     # Overwrite input file in place
  vrackconverter <input.mrk>             # Auto-create .vcv (never modifies .mrk)
  vrackconverter <input.vcv> -o output.mrk  # Convert V2 to MiRack
  vrackconverter <dir-with-mrk>         # Auto-creates .vcv in same directory
  vrackconverter <dir> -o <output>       # Batch convert directory

Arguments:
  <input>    Input .vcv or .mrk file, or directory

Flags:
  -o, --output <path>    Output file/directory (if not specified, requires --overwrite)
      --format <fmt>     Output format: v2, v06, or mirack (overrides file extension)
      --overwrite        Overwrite input file in place
  -m, --metamodule       Add 4ms MetaModule (HubMedium) to patch
  -q, --quiet            Suppress non-error output
  -V, --version          Show version
  -h, --help             Show this help

Examples:
  # MiRack to V2
  vrackconverter my-patch.mrk                      # Creates my-patch.vcv
  vrackconverter my-patch.mrk -o converted.vcv
  vrackconverter ./mrk-patches/                    # Creates .vcv alongside .mrk

  # V2 to MiRack (NEW)
  vrackconverter v2-patch.vcv -o output.mrk       # Creates output.mrk bundle
  vrackconverter ./v2-patches/ -o ./mirack/        # Batch convert to MiRack

  # Explicit format selection (NEW)
  vrackconverter input.vcv -o output.vcv --format v06  # Force v0.6 format
  vrackconverter input.vcv --overwrite --format v06     # In-place to v0.6

  # In-place
  vrackconverter old-patch.vcv --overwrite
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

// parseOutputFormat converts a format string to a Format type.
func parseOutputFormat(formatStr string) (converter.Format, error) {
	switch strings.ToLower(strings.TrimSpace(formatStr)) {
	case "v2", "vcv2", "2":
		return converter.FormatVCV2, nil
	case "v06", "v0.6", "vcv06", "vcv0.6", "0.6", "06":
		return converter.FormatVCV06, nil
	case "mirack", "mrk":
		return converter.FormatMiRack, nil
	default:
		return "", fmt.Errorf("invalid format: %s (must be v2, v06, or mirack)", formatStr)
	}
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
		case "--format":
			if i+1 < len(args) {
				outputFormat = args[i+1]
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

	// Parse output format if specified
	if outputFormat != "" {
		format, err := parseOutputFormat(outputFormat)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		opts.OutputFormat = format
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
		doConvert(inputPath, outputPath, opts)
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
		doConvertDirectory(inputPath, outputPath, opts)
		return
	}

	// For non-.mrk files
	if outputPath == "" && !overwrite {
		fmt.Fprintln(os.Stderr, "error: must specify -o <output> or --overwrite")
		fmt.Fprintln(os.Stderr, "  (to convert in place and overwrite the input file)")
		printUsage()
		os.Exit(1)
	}

	// In-place conversion: output = input
	if outputPath == "" && overwrite {
		outputPath = inputPath
	}

	doConvert(inputPath, outputPath, opts)
}

func doConvert(inputPath, outputPath string, opts converter.Options) {
	if !opts.Quiet {
		if inputPath == outputPath {
			fmt.Printf("Converting: %s (in place)\n", inputPath)
		} else {
			fmt.Printf("Converting: %s -> %s\n", inputPath, outputPath)
		}
	}

	result := converter.ConvertFile(inputPath, outputPath, opts)
	if result.Skipped {
		if len(result.Issues) > 0 {
			// Validation skip (e.g., MiRack audio module constraints)
			fmt.Fprintf(os.Stderr, "info: %s (skipped)\n", result.Issues[0])
		} else {
			fmt.Fprintf(os.Stderr, "info: file is already in target format (no conversion needed)\n")
		}
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

func doConvertDirectory(inputDir, outputDir string, opts converter.Options) {
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
				if len(result.Issues) > 0 {
					fmt.Printf("  ⊘ %s (%s)\n", relPath, result.Issues[0])
				} else {
					fmt.Printf("  ⊘ %s (already in target format)\n", relPath)
				}
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
