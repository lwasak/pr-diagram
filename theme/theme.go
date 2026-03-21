// Package theme holds all visual style constants for prdiagram.
// Edit this file to change the entire color scheme and font.
package theme

// ── Colors ───────────────────────────────────────────────────────────────────

const (
	// Page / diagram background.
	Background = "#282C34"

	// NOTE: D2 class shape swaps the usual CSS meaning:
	//   style.fill   → renders as the node BORDER colour
	//   style.stroke → renders as the node BACKGROUND colour
	// The names below use the visual meaning, not the D2 property name.

	// Background shared by all regular nodes.
	NodeFill       = "#282C34" // used as style.stroke in D2 (= visual background)
	NodeTitleColor = "#2C313C" // dark text on light background

	// Border colour per node kind (used as style.fill in D2).
	StrokeClass     = "#ABB2BF" // subtle — regular class
	StrokeAbstract  = "#61AFEF" // blue   — abstract class
	StrokeInterface = "#98C379" // green  — interface
	StrokeEnum      = "#56B6C2" // purple — enum
	StrokeRecord    = "#E06C75" // red    — record
	StrokeStruct    = "#ABB2BF" // orange — struct

	// Project containers — dashed outline only, no fill.
	ContainerFill   = "transparent"
	ContainerStroke = "#E5C07B"

	// Ghost / external nodes — rendered as rectangle shapes (not class shapes)
	// so fill/stroke behave as standard SVG: fill=background, stroke=border.
	GhostFill   = "#282C34" // dark background
	GhostStroke = "#E5C07B" // yellow dashed border + label text

	// Relationship arrows.
	ArrowExtends    = "#61AFEF" // blue  — inheritance (dashed)
	ArrowImplements = "#98C379" // green — interface impl (dashed)
	ArrowUses       = "#ABB2BF" // yellow — dependency (solid)

	// Member text (applied by JS in the HTML output).
	MemberVis            = "#E06C75" // red    — +, #, -, @
	MemberType           = "#C678DD" // purple — reference type names
	MemberValueType      = "#61AFEF" // blue   — value types (int, bool, struct, enum…)
	MemberCollectionType = "#E5C07B" // cyan   — collection / enumerable types
	MemberEnumType       = "#56B6C2" // purple — enum types   (matches StrokeEnum)
	MemberRecordType     = "#E06C75" // red    — record types (matches StrokeRecord)
	MemberStructType     = "#ABB2BF" // orange — struct types
	MemberGeneric        = "#98C379" // green  — generic punctuation <, >, ,
	MemberNullable       = "#ABB2BF" // yellow — nullable suffix ?
	MemberName           = "#ABB2BF" // gray   — member / method names
)

// ── Font ─────────────────────────────────────────────────────────────────────

const (
	FontFamily     = "'JetBrains Mono', monospace"
	FontFamilyCSS  = "'JetBrains Mono', monospace"
	FontGoogleHref = "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap"
)
