package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// TypeInfo mirrors the JSON contract emitted by the .NET analyzer.
type TypeInfo struct {
	Name         string     `json:"name"`
	Kind         string     `json:"kind"` // class | interface | enum | record | struct
	IsExternal   bool       `json:"isExternal"`
	IsAbstract   bool       `json:"isAbstract"`
	IsInternal   bool       `json:"isInternal"`
	Visibility   string     `json:"visibility"` // public | protected | internal | private
	BaseType     string     `json:"baseType"`
	Interfaces   []string   `json:"interfaces"`
	Properties   []Property `json:"properties"`
	Methods      []Method   `json:"methods"`
	Members      []string   `json:"members"`      // enum member names
	MemberValues []string   `json:"memberValues"` // enum member numeric values (parallel to Members)
	Dependencies []string   `json:"dependencies"` // all referenced type names (incl. private fields & ctors)
	SourceFile      string     `json:"sourceFile"`      // absolute path of the source file (empty for ghost nodes)
	TypeParameters  []string   `json:"typeParameters"`  // generic type param names, e.g. ["T", "TValue"]
	Project         string     `json:"-"`               // project name derived from nearest .csproj; set by main
}

type Property struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Visibility string `json:"visibility"`
}

type Method struct {
	Name       string      `json:"name"`
	ReturnType string      `json:"returnType"`
	Visibility string      `json:"visibility"`
	Parameters []Parameter `json:"parameters"`
}

type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// AnalyzerExePath returns the path to the self-contained analyzer binary
// relative to the current executable (or CWD during development).
func AnalyzerExePath() string {
	// Try next to the running executable first (production layout)
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		p := analyzerPath(exeDir)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fall back to CWD (development: run from project root)
	cwd, _ := os.Getwd()
	return analyzerPath(cwd)
}

func analyzerPath(base string) string {
	name := "analyzer"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(base, "analyzer", "dist", name)
}

// RunAnalyzer invokes the self-contained .NET analyzer binary and returns
// the parsed []TypeInfo.  csPaths must be absolute or resolvable paths to
// the .cs files to analyse.
func RunAnalyzer(analyzerExe string, csPaths []string) ([]TypeInfo, error) {
	if len(csPaths) == 0 {
		return nil, fmt.Errorf("no .cs files provided")
	}

	args := append([]string{}, csPaths...)
	cmd := exec.Command(analyzerExe, args...)

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("analyzer failed: %s", msg)
	}

	var types []TypeInfo
	if err := json.Unmarshal([]byte(stdout.String()), &types); err != nil {
		return nil, fmt.Errorf("failed to parse analyzer output: %w\noutput was: %s", err, stdout.String())
	}
	return types, nil
}
