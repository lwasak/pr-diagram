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
	NodeTitleColor = "#2C313C"

	// Border colour per node kind (used as style.fill in D2).
	StrokeClass     = "#ABB2BF" // regular class
	StrokeAbstract  = "#61AFEF" // abstract class
	StrokeInterface = "#98C379" // interface
	StrokeEnum      = "#56B6C2" // enum
	StrokeRecord    = "#E06C75" // record
	StrokeStruct    = "#ABB2BF" // struct

	// Project containers — dashed outline only, no fill.
	ContainerFill   = "transparent"
	ContainerStroke = "#E5C07B"

	// Ghost / external nodes — rendered as rectangle shapes (not class shapes)
	// so fill/stroke behave as standard SVG: fill=background, stroke=border.
	GhostFill   = "#282C34"
	GhostStroke = "#E5C07B"

	// Relationship arrows.
	ArrowExtends    = "#61AFEF" // inheritance (dashed)
	ArrowImplements = "#98C379" // interface impl (dashed)
	ArrowUses       = "#ABB2BF" // dependency (solid)

	// Member text (applied by JS in the HTML output).
	MemberVis            = "#E06C75" // +, #, -, @
	MemberType           = "#C678DD" // reference type names
	MemberValueType      = "#61AFEF" // value types (int, bool, struct, enum…)
	MemberCollectionType = "#E5C07B" // collection / enumerable types
	MemberEnumType       = "#56B6C2" // enum types   (matches StrokeEnum)
	MemberRecordType     = "#E06C75" // record types (matches StrokeRecord)
	MemberStructType     = "#ABB2BF" // struct types
	MemberGeneric        = "#98C379" // generic punctuation <, >, ,
	MemberNullable       = "#ABB2BF" // nullable suffix ?
	MemberName           = "#ABB2BF" // member / method names
)

// ── Font ─────────────────────────────────────────────────────────────────────

const (
	FontFamily     = "'JetBrains Mono', monospace"
	FontFamilyCSS  = "'JetBrains Mono', monospace"
	FontGoogleHref = "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap"
)
