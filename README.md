# pr-diagram

CLI tool that turns C# source files from an Azure DevOps PR into a class-relationship diagram — rendered as an SVG embedded in a self-contained HTML page.

> **Screenshot / demo** — add one here once you have a representative diagram.

---

## Requirements

| Tool | Version |
|------|---------|
| Go | 1.24+ |
| .NET SDK | 10.0+ (only needed to build the Roslyn analyzer) |

The compiled analyzer binary is **not** committed to the repo, so you must build it once before first use.

---

## Quick start

```sh
git clone https://github.com/lwasak/pr-diagram
cd pr-diagram

# Build both the Go binary and the .NET Roslyn analyzer
./build.sh          # Linux / macOS
# or
./build.ps1         # Windows (PowerShell)

# Run against the bundled examples
./prdiagram --dir examples
# Opens pr-diagram-local.html in your default browser
```

---

## Usage

```
prdiagram [flags]

Flags:
  --files    Comma-separated list of .cs file paths to analyze
  --dir      Directory — all .cs files inside are parsed recursively
  --out      Output directory for the HTML file (default: current directory)
  --dry-run  Print the D2 diagram source to stdout instead of rendering
  --analyzer Path to the analyzer binary (default: auto-detected)
```

### Examples

```sh
# Single directory
prdiagram --dir src/MyProject

# Specific files
prdiagram --files "src/Domain/Order.cs,src/Domain/Customer.cs"

# Write HTML to a specific directory
prdiagram --dir src/MyProject --out /tmp/diagrams

# Inspect the raw D2 source without rendering
prdiagram --dir src/MyProject --dry-run
```

---

## How it works

```
.cs files
   │
   ▼
Roslyn analyzer (.NET, single-file exe)
   │  Parses syntax tree, emits JSON:
   │  types, properties, methods, inheritance, interfaces
   ▼
D2 diagram source (Go)
   │  Builds nodes and edges for classes, interfaces, enums
   ▼
SVG via D2 v0.7.1 (Go library, DarkMauve theme)
   │
   ▼
Self-contained HTML page
   (interactive: hover/click to highlight related nodes)
```

The Roslyn analyzer runs as a subprocess. Its output is JSON written to stdout; no temp files are used.

---

## Building

### Windows

```powershell
./build.ps1
```

Produces:
- `analyzer/dist/analyzer.exe` — trimmed, self-contained .NET 10 binary (~21 MB)
- `prdiagram.exe` — Go CLI binary

### Linux / macOS

```sh
./build.sh
```

`build.sh` auto-detects the .NET runtime identifier (`linux-x64`, `osx-arm64`, etc.).

Produces:
- `analyzer/dist/analyzer` — trimmed, self-contained binary
- `prdiagram` — Go CLI binary

---

## Project structure

```
pr-diagram/
├── main.go               # CLI entry point (flags, orchestration)
├── go.mod / go.sum       # Go module (github.com/lwasak/pr-diagram)
├── build.ps1             # Windows build script
├── build.sh              # Linux/macOS build script
├── parser/
│   └── runner.go         # Invokes analyzer, unmarshals JSON → TypeInfo structs
├── diagram/
│   └── renderer.go       # Builds D2 source, renders SVG
├── output/
│   └── html.go           # Wraps SVG in self-contained HTML, opens browser
├── theme/                # D2 colour/style constants
├── analyzer/
│   ├── Program.cs        # Roslyn syntax parser, JSON emitter
│   ├── analyzer.csproj   # net10.0, PublishSingleFile, trimmed
│   └── TrimmerRoots.xml  # Trim configuration for Roslyn
└── examples/             # 16 .cs files exercising all diagram features
```

---

## Roadmap

- [ ] Azure DevOps integration — `--org`, `--project`, `--repo`, `--pr`, `--token` flags to fetch changed files directly from a PR
- [ ] `--expand-deps` — fetch and diagram external type definitions
- [ ] HTML diff summary alongside the diagram

---

## License

[MIT](LICENSE) © Łukasz Wasak
