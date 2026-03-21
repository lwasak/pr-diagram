package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lwasak/pr-diagram/diagram"
	"github.com/lwasak/pr-diagram/output"
	"github.com/lwasak/pr-diagram/parser"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		filesFlag    = flag.String("files", "", "Comma-separated .cs file paths")
		dirFlag      = flag.String("dir", "", "Directory — all .cs files inside are parsed")
		outFlag      = flag.String("out", ".", "Output directory for the HTML file")
		dryRun       = flag.Bool("dry-run", false, "Print D2 source to stdout instead of opening browser")
		analyzerFlag = flag.String("analyzer", "", "Path to analyzer binary (default: auto-detect)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `pr-diagram — generate a C# class-relationship diagram from source files

Usage:
  prdiagram --dir <path>            Diagram all .cs files in a directory
  prdiagram --files <a.cs,b.cs>     Diagram specific files

Flags:
  --dir       <path>   Directory to scan recursively for .cs files
  --files     <paths>  Comma-separated list of .cs file paths
  --out       <path>   Output directory for the HTML file (default: .)
  --dry-run            Print D2 diagram source to stdout; do not render
  --analyzer  <path>   Path to analyzer binary (default: auto-detect)
  --help               Show this help message

Examples:
  prdiagram --dir src/MyProject
  prdiagram --files "src/Order.cs,src/Customer.cs"
  prdiagram --dir src/MyProject --out /tmp/diagrams
  prdiagram --dir src/MyProject --dry-run
`)
	}

	flag.Parse()

	// ── Collect .cs file paths ────────────────────────────────────────────────
	var csPaths []string

	if *filesFlag != "" {
		for _, p := range strings.Split(*filesFlag, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				abs, err := filepath.Abs(p)
				if err != nil {
					return fmt.Errorf("bad path %q: %w", p, err)
				}
				csPaths = append(csPaths, abs)
			}
		}
	}

	if *dirFlag != "" {
		err := filepath.WalkDir(*dirFlag, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".cs") {
				abs, _ := filepath.Abs(path)
				csPaths = append(csPaths, abs)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walking dir %q: %w", *dirFlag, err)
		}
	}

	// Build dir → project name map by scanning for .csproj files.
	// Project name = csproj filename without extension (e.g. "PRExample.Domain").
	projectByDir := map[string]string{}
	if *dirFlag != "" {
		filepath.WalkDir(*dirFlag, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
			if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".csproj") {
				absDir, _ := filepath.Abs(filepath.Dir(path))
				projectByDir[absDir] = strings.TrimSuffix(d.Name(), ".csproj")
			}
			return nil
		})
	}

	if len(csPaths) == 0 {
		flag.Usage()
		return fmt.Errorf("provide --files or --dir with at least one .cs file")
	}

	// ── Locate analyzer binary ────────────────────────────────────────────────
	analyzerExe := *analyzerFlag
	if analyzerExe == "" {
		analyzerExe = parser.AnalyzerExePath()
	}
	if _, err := os.Stat(analyzerExe); err != nil {
		return fmt.Errorf("analyzer binary not found at %q — run build.ps1 first", analyzerExe)
	}

	// ── Parse → render ────────────────────────────────────────────────────────
	fmt.Fprintf(os.Stderr, "Analyzing %d file(s)...\n", len(csPaths))
	types, err := parser.RunAnalyzer(analyzerExe, csPaths)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Found %d types (%d external).\n",
		len(types), countExternal(types))

	// Assign project name to each type from its source file's directory.
	for i := range types {
		if types[i].SourceFile == "" {
			continue
		}
		absDir, _ := filepath.Abs(filepath.Dir(types[i].SourceFile))
		types[i].Project = projectByDir[absDir]
	}

	if *dryRun {
		fmt.Print(diagram.RenderD2Source(types))
		return nil
	}

	fmt.Fprintf(os.Stderr, "Rendering diagram...\n")
	svg, err := diagram.Render(types)
	if err != nil {
		return err
	}

	typeKinds := make(map[string]string, len(types))
	for _, t := range types {
		if !t.IsExternal {
			typeKinds[t.Name] = t.Kind
		}
	}

	edges := diagram.ExtractEdges(types)
	outPath, err := output.WriteHTML(svg, "local", *outFlag, typeKinds, edges)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Diagram written to %s\n", outPath)
	return nil
}

func countExternal(types []parser.TypeInfo) int {
	n := 0
	for _, t := range types {
		if t.IsExternal {
			n++
		}
	}
	return n
}
