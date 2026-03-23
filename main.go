package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lwasak/pr-diagram/azdevops"
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
		prFlag       = flag.Int("pr", 0, "Azure DevOps Pull Request number")
		orgFlag      = flag.String("org", "", "Azure DevOps organisation name")
		projectFlag  = flag.String("project", "", "Azure DevOps project name")
		tokenFlag    = flag.String("token", "", "Azure DevOps PAT (or set AZURE_DEVOPS_TOKEN env var)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `pr-diagram — generate a C# class-relationship diagram from source files

Usage:
  prdiagram --dir <path>                              Diagram all .cs files in a directory
  prdiagram --files <a.cs,b.cs>                       Diagram specific files
  prdiagram --pr <number> --org <o> --project <p>     Diagram a PR from Azure DevOps

Flags:
  --dir       <path>     Directory to scan recursively for .cs files
  --files     <paths>    Comma-separated list of .cs file paths
  --out       <path>     Output directory for the HTML file (default: .)
  --dry-run              Print D2 diagram source to stdout; do not render
  --analyzer  <path>     Path to analyzer binary (default: auto-detect)
  --pr        <number>   Azure DevOps Pull Request number
  --org       <name>     Azure DevOps organisation name
  --project   <name>     Azure DevOps project name
  --token     <PAT>      Personal Access Token (or set AZURE_DEVOPS_TOKEN env var)
  --help                 Show this help message

Examples:
  prdiagram --dir src/MyProject
  prdiagram --files "src/Order.cs,src/Customer.cs"
  prdiagram --dir src/MyProject --out /tmp/diagrams
  prdiagram --dir src/MyProject --dry-run
  prdiagram --pr 123 --org myorg --project myproject --token <PAT>
  AZURE_DEVOPS_TOKEN=<PAT> prdiagram --pr 123 --org myorg --project myproject
`)
	}

	flag.Parse()

	// Resolve token with env var fallback.
	token := *tokenFlag
	if token == "" {
		token = os.Getenv("AZURE_DEVOPS_TOKEN")
	}

	// ── PR mode: fetch files from Azure DevOps ────────────────────────────────
	if *prFlag != 0 {
		if *filesFlag != "" || *dirFlag != "" {
			return fmt.Errorf("--pr cannot be combined with --files or --dir")
		}
		tmpDir, err := fetchPRFiles(*prFlag, *orgFlag, *projectFlag, token)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		// Point dirFlag at tmpDir so both the .cs walk and .csproj scanning below pick it up.
		*dirFlag = tmpDir
	}

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
		if *prFlag != 0 {
			return fmt.Errorf("PR %d has no .cs files", *prFlag)
		}
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
	label := "local"
	if *prFlag != 0 {
		label = fmt.Sprintf("pr-%d", *prFlag)
	}
	outPath, err := output.WriteHTML(svg, label, *outFlag, typeKinds, edges)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Diagram written to %s\n", outPath)
	return nil
}

func fetchPRFiles(prID int, org, project, token string) (tmpDir string, err error) {
	if org == "" {
		return "", fmt.Errorf("--org is required when using --pr")
	}
	if project == "" {
		return "", fmt.Errorf("--project is required when using --pr")
	}
	if token == "" {
		return "", fmt.Errorf("--token is required when using --pr (or set AZURE_DEVOPS_TOKEN)")
	}

	ctx := context.Background()
	client := azdevops.NewClient(org, project, token)

	details, err := client.GetPRDetails(ctx, prID)
	if err != nil {
		return "", fmt.Errorf("getting PR details: %w", err)
	}
	client.SetRepo(details.Repository.Name)
	commitID := details.LastMergeSourceCommit.CommitID

	iterationID, err := client.GetLatestIterationID(ctx, prID)
	if err != nil {
		return "", fmt.Errorf("getting PR iterations: %w", err)
	}

	entries, err := client.GetChangedFiles(ctx, prID, iterationID)
	if err != nil {
		return "", fmt.Errorf("getting PR changes: %w", err)
	}

	// Filter: keep only added/modified .cs and .csproj files.
	var keep []azdevops.ChangeEntry
	for _, e := range entries {
		if strings.EqualFold(e.ChangeType, "delete") {
			continue
		}
		if strings.HasSuffix(e.Item.Path, ".cs") || strings.HasSuffix(e.Item.Path, ".csproj") {
			keep = append(keep, e)
		}
	}
	if len(keep) == 0 {
		return "", fmt.Errorf("PR %d has no .cs or .csproj changes", prID)
	}

	fmt.Fprintf(os.Stderr, "Downloading %d file(s) from PR #%d...\n", len(keep), prID)

	tmpDir, err = os.MkdirTemp("", "pr-diagram-*")
	if err != nil {
		return "", err
	}

	for _, e := range keep {
		data, dlErr := client.DownloadFile(ctx, e.Item.Path, commitID)
		if dlErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", e.Item.Path, dlErr)
			continue
		}
		// Strip leading "/" and convert to OS path separators.
		rel := strings.TrimPrefix(e.Item.Path, "/")
		localPath := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(localPath), 0o755); mkErr != nil {
			return tmpDir, mkErr
		}
		if writeErr := os.WriteFile(localPath, data, 0o644); writeErr != nil {
			return tmpDir, writeErr
		}
	}

	return tmpDir, nil
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
