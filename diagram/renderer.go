package diagram

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"log/slog"
	"os"

	"github.com/lwasak/pr-diagram/parser"
	"github.com/lwasak/pr-diagram/theme"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

// Render converts a slice of TypeInfo to a D2-rendered SVG.
func Render(types []parser.TypeInfo) ([]byte, error) {
	src := RenderD2Source(types)

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("init font ruler: %w", err)
	}

	themeID := d2themescatalog.DarkMauve.ID
	pad := int64(d2svg.DEFAULT_PADDING)

	renderOpts := &d2svg.RenderOpts{
		ThemeID: &themeID,
		Pad:     &pad,
	}

	// Suppress d2's internal debug logs by injecting a discard logger.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := d2log.With(context.Background(), logger)

	diagram, _, err := d2lib.Compile(ctx, src, &d2lib.CompileOptions{
		Ruler: ruler,
		LayoutResolver: func(_ string) (d2graph.LayoutGraph, error) {
			return func(ctx context.Context, g *d2graph.Graph) error {
				return d2elklayout.Layout(ctx, g, nil)
			}, nil
		},
	}, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("d2 compile: %w", err)
	}

	svg, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("d2 render: %w", err)
	}
	return svg, nil
}

// Edge represents a directed relationship between two types.
type Edge struct {
	From string
	To   string
}

// ExtractEdges returns all edges that RenderD2Source would draw,
// using the same node-existence and deduplication logic.
func ExtractEdges(types []parser.TypeInfo) []Edge {
	nodeNames := make(map[string]bool, len(types))
	for _, t := range types {
		nodeNames[t.Name] = true
	}

	emitted := make(map[string]bool)
	structural := make(map[string]bool)
	var edges []Edge

	emit := func(from, to string) {
		key := from + "→" + to
		if emitted[key] {
			return
		}
		emitted[key] = true
		structural[from+"→"+to] = true
		edges = append(edges, Edge{From: from, To: to})
	}

	for _, t := range types {
		if t.IsExternal {
			continue
		}
		if t.BaseType != "" && nodeNames[t.BaseType] {
			emit(t.Name, t.BaseType)
		}
		for _, iface := range t.Interfaces {
			if nodeNames[iface] {
				emit(t.Name, iface)
			}
		}
		for _, dep := range t.Dependencies {
			r := rootType(dep)
			if !nodeNames[r] || r == t.Name {
				continue
			}
			if structural[t.Name+"→"+r] {
				continue
			}
			key := "uses:" + t.Name + "→" + r
			if emitted[key] {
				continue
			}
			emitted[key] = true
			edges = append(edges, Edge{From: t.Name, To: r})
		}
	}
	return edges
}

// RenderD2Source converts TypeInfo slices into D2 diagram source.
// Exported so main can use it for --dry-run.
func RenderD2Source(types []parser.TypeInfo) string {
	nodeNames := make(map[string]bool, len(types))
	nodeProject := make(map[string]string) // type name → project name
	for _, t := range types {
		nodeNames[t.Name] = true
		if t.Project != "" {
			nodeProject[t.Name] = t.Project
		}
	}

	// d2Path returns the qualified D2 path for use in edge declarations.
	// Nodes inside a project container must be referenced as "Project.NodeID".
	d2Path := func(name string) string {
		if proj := nodeProject[name]; proj != "" {
			return d2ID(proj) + "." + d2ID(name)
		}
		return d2ID(name)
	}

	// Collect ordered unique project names (preserve first-seen order).
	seenProjects := make(map[string]bool)
	var projects []string
	for _, t := range types {
		if t.Project != "" && !seenProjects[t.Project] {
			seenProjects[t.Project] = true
			projects = append(projects, t.Project)
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "direction: right\n\n")

	// ── External (ghost) nodes — top-level, outside project containers ────────
	for _, t := range types {
		if t.IsExternal {
			writeGhostNode(&buf, t.Name)
		}
	}

	// ── Project containers ─────────────────────────────────────────────────────
	for _, proj := range projects {
		fmt.Fprintf(&buf, "%s: {\n  label: %q\n  style.fill: %q\n  style.stroke: %q\n  style.stroke-dash: 4\n  style.border-radius: 8\n\n",
			d2ID(proj), proj, theme.ContainerFill, theme.ContainerStroke)
		for _, t := range types {
			if t.IsExternal || t.Project != proj {
				continue
			}
			switch t.Kind {
			case "enum":
				writeEnumNode(&buf, t)
			case "interface":
				writeInterfaceNode(&buf, t)
			default:
				writeClassNode(&buf, t)
			}
		}
		fmt.Fprintf(&buf, "}\n\n")
	}

	// ── Ungrouped regular nodes (no project assigned) ──────────────────────────
	for _, t := range types {
		if t.IsExternal || t.Project != "" {
			continue
		}
		switch t.Kind {
		case "enum":
			writeEnumNode(&buf, t)
		case "interface":
			writeInterfaceNode(&buf, t)
		default:
			writeClassNode(&buf, t)
		}
	}

	// ── Relationship edges ────────────────────────────────────────────────────
	colorExtends    := theme.ArrowExtends
	colorImplements := theme.ArrowImplements
	colorUses       := theme.ArrowUses

	emitted    := make(map[string]bool)
	structural := make(map[string]bool) // pairs with extends/implements — suppress uses

	edge := func(from, to, label string, dashed bool) {
		key := from + "→" + to + label
		if emitted[key] {
			return
		}
		emitted[key] = true
		structural[from+"→"+to] = true
		if dashed {
			fmt.Fprintf(&buf,
				"%s <- %s: %s {\n  style.stroke-dash: 4\n  style.stroke: %q\n  style.font-color: %q\n}\n",
				d2Path(to), d2Path(from), label, colorImplements, colorImplements)
		} else {
			fmt.Fprintf(&buf,
				"%s <- %s: %s {\n  style.stroke-dash: 2\n  style.stroke: %q\n  style.font-color: %q\n}\n",
				d2Path(to), d2Path(from), label, colorExtends, colorExtends)
		}
	}

	usesEdge := func(owner, target string) {
		if !nodeNames[target] || owner == target {
			return
		}
		if structural[owner+"→"+target] {
			return
		}
		key := "uses:" + owner + "→" + target
		if emitted[key] {
			return
		}
		emitted[key] = true
		fmt.Fprintf(&buf,
			"%s -> %s: uses {\n  style.stroke: %q\n  style.font-color: %q\n}\n",
			d2Path(owner), d2Path(target), colorUses, colorUses)
	}

	for _, t := range types {
		if t.IsExternal {
			continue
		}
		if t.BaseType != "" && nodeNames[t.BaseType] {
			edge(t.Name, t.BaseType, "extends", false)
		}
		for _, iface := range t.Interfaces {
			if nodeNames[iface] {
				edge(t.Name, iface, "implements", true)
			}
		}
		// Emit uses edges from all referenced types (public members + private fields + ctors).
		for _, dep := range t.Dependencies {
			usesEdge(t.Name, rootType(dep))
		}
	}

	return buf.String()
}

// ── Node writers ──────────────────────────────────────────────────────────────

// nodeOpen writes the opening of a D2 class shape node with theme styling.
// label is shown as the node title (e.g. "+ MyClass"); id is the D2 key.
func nodeOpen(buf *bytes.Buffer, id, label, stroke string) {
	// In D2 class shapes style.fill colours the border and style.stroke colours
	// the background — the opposite of conventional CSS/SVG naming.
	fmt.Fprintf(buf, "%s: %s {\n  shape: class\n  style.fill: %q\n  style.stroke: %q\n  style.font-color: %q\n",
		id, strconv.Quote(label), stroke, theme.NodeFill, theme.NodeTitleColor)
}

// typeVisLabel returns the UML visibility prefix for a type's access modifier.
func typeVisLabel(vis string) string {
	switch vis {
	case "public":
		return "+"
	case "protected":
		return "#"
	case "internal":
		return "~"
	case "private":
		return "-"
	default:
		return "+"
	}
}

func writeClassNode(buf *bytes.Buffer, t parser.TypeInfo) {
	maxLen := 0
	for _, p := range t.Properties {
		if l := len(visSymbol(p.Visibility) + p.Type); l > maxLen {
			maxLen = l
		}
	}
	for _, m := range t.Methods {
		if l := len(visSymbol(m.Visibility) + m.ReturnType); l > maxLen {
			maxLen = l
		}
	}
	stroke := theme.StrokeClass
	switch {
	case t.IsAbstract:
		stroke = theme.StrokeAbstract
	case t.Kind == "record":
		stroke = theme.StrokeRecord
	case t.Kind == "struct":
		stroke = theme.StrokeStruct
	}
	nodeOpen(buf, d2ID(t.Name), typeVisLabel(t.Visibility)+" "+t.Name+genericSuffix(t.TypeParameters), stroke)
	for _, p := range t.Properties {
		prefix := visSymbol(p.Visibility) + p.Type
		fmt.Fprintf(buf, "  %s: \"\"\n", d2Quote(padTo(prefix, maxLen)+"\u00a0"+p.Name))
	}
	for _, m := range t.Methods {
		prefix := visSymbol(m.Visibility) + m.ReturnType
		fmt.Fprintf(buf, "  %s: \"\"\n", d2Quote(padTo(prefix, maxLen)+"\u00a0"+m.Name+"("+fmtParams(m.Parameters)+")"))
	}
	fmt.Fprintf(buf, "}\n")
}

func writeInterfaceNode(buf *bytes.Buffer, t parser.TypeInfo) {
	maxLen := 0
	for _, p := range t.Properties {
		if l := len("+" + p.Type); l > maxLen {
			maxLen = l
		}
	}
	for _, m := range t.Methods {
		if l := len("+" + m.ReturnType); l > maxLen {
			maxLen = l
		}
	}
	nodeOpen(buf, d2ID(t.Name), typeVisLabel(t.Visibility)+" "+t.Name+genericSuffix(t.TypeParameters), theme.StrokeInterface)
	for _, p := range t.Properties {
		prefix := "+" + p.Type
		fmt.Fprintf(buf, "  %s: \"\"\n", d2Quote(padTo(prefix, maxLen)+"\u00a0"+p.Name))
	}
	for _, m := range t.Methods {
		prefix := "+" + m.ReturnType
		fmt.Fprintf(buf, "  %s: \"\"\n", d2Quote(padTo(prefix, maxLen)+"\u00a0"+m.Name+"("+fmtParams(m.Parameters)+")"))
	}
	fmt.Fprintf(buf, "}\n")
}

func writeEnumNode(buf *bytes.Buffer, t parser.TypeInfo) {
	nodeOpen(buf, d2ID(t.Name), typeVisLabel(t.Visibility)+" "+t.Name, theme.StrokeEnum)
	for i, name := range t.Members {
		val := strconv.Itoa(i)
		if i < len(t.MemberValues) {
			val = t.MemberValues[i]
		}
		fmt.Fprintf(buf, "  %s: \"\"\n", d2Quote(val+"\u00a0\u00a0"+name))
	}
	fmt.Fprintf(buf, "}\n")
}

func writeGhostNode(buf *bytes.Buffer, name string) {
	// Use rectangle shape (not class) so fill/stroke behave as normal SVG:
	//   style.fill   = background  (dark)
	//   style.stroke = border line (yellow dashed)
	// D2 class shapes conflate style.fill with the header rect fill, making it
	// impossible to have a dark body with a coloured border on member-less nodes.
	// Pad with non-breaking spaces (U+00A0) for visual side padding — D2
	// rectangles size tightly around the label and U+00A0 is never collapsed.
	// d2Quote passes U+00A0 through as a literal character (unlike Go's %q which
	// would escape it as \u00a0 and break the padding in D2's parser).
	label := d2Quote("\u00a0\u00a0\u00a0" + name + "\u00a0\u00a0\u00a0")
	// D2's text ruler underestimates JetBrains Mono width at font-size 24, causing
	// long names to clip. Set an explicit minimum width: ~14.5px per char + 48px
	// side padding, ensuring even the longest type names have room to breathe.
	minWidth := len(name)*145/10 + 48
	fmt.Fprintf(buf,
		"%s: %s {\n  shape: rectangle\n  width: %d\n  style.fill: %q\n  style.stroke-dash: 8\n  style.stroke-width: 2\n  style.stroke: %q\n  style.font-color: %q\n  style.font-size: 24\n  style.border-radius: 4\n}\n",
		d2ID(name), label, minWidth, theme.GhostFill, theme.GhostStroke, theme.GhostStroke)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// padTo pads s with non-breaking spaces (U+00A0) to width. Regular ASCII spaces
// are collapsed by SVG/HTML whitespace rules when SVG is embedded in HTML;
// U+00A0 is always preserved, so column alignment is stable in the browser.
func padTo(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat("\u00a0", width-len(s))
}

func visSymbol(vis string) string {
	switch vis {
	case "public":
		return "+"
	case "protected":
		return "#"
	case "internal":
		return "-" // D2 doesn't support "~" as a vis modifier; JS remaps "-" → "~"
	default:
		return "-"
	}
}

func fmtParams(params []parser.Parameter) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = p.Type + " " + p.Name
	}
	return strings.Join(parts, ", ")
}

func rootType(t string) string {
	t = strings.TrimSuffix(t, "?")
	if idx := strings.IndexByte(t, '<'); idx >= 0 {
		return t[:idx]
	}
	return t
}

// genericSuffix returns "<T>" / "<T, U>" etc. for types with type parameters,
// or "" for non-generic types.
func genericSuffix(params []string) string {
	if len(params) == 0 {
		return ""
	}
	return "<" + strings.Join(params, ", ") + ">"
}

// d2Quote wraps s in D2 double-quotes, escaping only backslash and double-quote.
// Unlike Go's %q, it does NOT escape non-ASCII characters, so U+00A0 padding
// reaches the SVG text element intact and is never collapsed by the browser.
func d2Quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func d2ID(name string) string {
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			return fmt.Sprintf("%q", name)
		}
	}
	return name
}
