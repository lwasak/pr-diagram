package output

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/lwasak/pr-diagram/diagram"
	"github.com/lwasak/pr-diagram/theme"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link href="%s" rel="stylesheet">
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { width: 100%%; background: %s; color: %s; font-family: %s; }
    svg text { font-family: %s !important; }
    .diagram-wrap {
      position: relative;
      overflow: hidden;
      cursor: grab;
      height: calc(100vh - 180px);
      min-height: 300px;
      user-select: none;
    }
    .diagram-wrap.panning { cursor: grabbing; }
    .diagram-wrap svg {
      display: block;
      transform-origin: 0 0;
    }
    .zoom-bar {
      position: fixed;
      bottom: 20px;
      right: 20px;
      display: flex;
      align-items: center;
      gap: 4px;
      background: #2C313C;
      border-radius: 6px;
      padding: 4px 8px;
      font-size: 12px;
      z-index: 10;
      box-shadow: 0 2px 8px rgba(0,0,0,.4);
    }
    .zoom-bar button {
      background: none;
      border: none;
      color: #abb2bf;
      font-size: 16px;
      cursor: pointer;
      padding: 2px 6px;
      border-radius: 4px;
      line-height: 1;
    }
    .zoom-bar button:hover { background: #3E4452; color: #fff; }
    .zoom-bar span { color: #abb2bf; min-width: 44px; text-align: center; }
    .legend {
      display: flex;
      flex-wrap: wrap;
      justify-content: center;
      gap: 10px;
      padding: 14px 24px 24px;
      font-size: 12px;
    }
    .legend-section {
      background: #2C313C;
      border-radius: 6px;
      padding: 9px 13px 11px;
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    .legend-section-title {
      font-size: 10px;
      font-weight: 700;
      color: #555;
      text-transform: uppercase;
      letter-spacing: .07em;
      margin-bottom: 1px;
    }
    .legend-item {
      display: flex;
      align-items: center;
      gap: 7px;
    }
    .legend-item span { color: #ccc; }
    .legend-ghost {
      display: block;
      width: 13px;
      height: 13px;
      border: 2px dashed;
      border-radius: 2px;
      flex-shrink: 0;
    }
  </style>
</head>
<body>
  <div class="diagram-wrap">
    %s
  </div>
  <script>
  // Colorize class member text in the embedded D2 SVG.
  //
  // D2 renders class members in TWO separate SVG <text> elements:
  //   1. The visibility symbol (+, #, -) alone
  //   2. The rest of the key text (type + U+00A0 padding + name/params)
  //
  // We use U+00A0 (non-breaking space) as the separator between the type and
  // name columns — it is never collapsed and lets us split reliably.
  document.addEventListener('DOMContentLoaded', () => {
    const NS              = 'http://www.w3.org/2000/svg';
    const BACKGROUND           = '%s'; // page/diagram background
    const VIS_COLOR            = '%s'; // +, #, ~
    const TYPE_COLOR           = '%s'; // reference type names
    const VALUE_TYPE_COLOR     = '%s'; // value types (int, bool, struct, enum…)
    const COLLECTION_TYPE_COLOR= '%s'; // collection / enumerable types
    const ENUM_TYPE_COLOR      = '%s'; // enum types
    const RECORD_TYPE_COLOR    = '%s'; // record types
    const STRUCT_TYPE_COLOR    = '%s'; // struct types
    const GENERIC_COLOR        = '%s'; // generic punctuation <, >, ,
    const NULLABLE_COLOR       = '%s'; // nullable suffix ?
    const NAME_COLOR           = '%s'; // member/method names

    // Fix D2's embedded background rect to match page background
    document.querySelectorAll('svg').forEach(svg => {
      const bg = svg.querySelector(':scope > rect');
      if (bg) bg.setAttribute('fill', BACKGROUND);
    });

    // C# value types that get VALUE_TYPE_COLOR instead of TYPE_COLOR
    const VALUE_TYPES = new Set([
      'bool','byte','sbyte','char','decimal','double','float',
      'int','uint','long','ulong','short','ushort','nint','nuint',
      'void','Guid','DateTime','DateTimeOffset','TimeSpan','DateOnly','TimeOnly',
    ]);

    // Collection / enumerable types that get COLLECTION_TYPE_COLOR
    const COLLECTION_TYPES = new Set([
      'IEnumerable','ICollection','IList','IReadOnlyList','IReadOnlyCollection',
      'ISet','IReadOnlySet',
      'IDictionary','IReadOnlyDictionary',
      'List','Dictionary','HashSet','SortedSet','SortedDictionary',
      'Queue','Stack','LinkedList','PriorityQueue',
      'Array','Span','Memory','ReadOnlySpan','ReadOnlyMemory',
      'ImmutableList','ImmutableArray','ImmutableDictionary','ImmutableHashSet',
      'ConcurrentBag','ConcurrentQueue','ConcurrentStack','ConcurrentDictionary',
    ]);

    // Type sets generated from parsed source — injected at render time.
    %s

    function mkTspan(text, fill) {
      const t = document.createElementNS(NS, 'tspan');
      t.textContent = text;
      if (fill) t.setAttribute('fill', fill);
      return t;
    }

    // Split "T1, T2, Nested<A,B>" at top-level commas only.
    function splitTopLevelArgs(inner) {
      const parts = [];
      let depth = 0, start = 0;
      for (let i = 0; i < inner.length; i++) {
        if      (inner[i] === '<') depth++;
        else if (inner[i] === '>') depth--;
        else if (inner[i] === ',' && depth === 0) {
          parts.push(inner.slice(start, i));
          start = i + 1;
        }
      }
      parts.push(inner.slice(start));
      return parts;
    }

    // Color a type string recursively.
    // <, >, , → GENERIC_COLOR; each type arg recurses for its own category color.
    function appendType(parent, typeText) {
      typeText = typeText.trim();
      // Handle nullable suffix
      const nullable = typeText.endsWith('?');
      if (nullable) typeText = typeText.slice(0, -1);

      const lt = typeText.indexOf('<');
      if (lt === -1) {
        const color = COLLECTION_TYPES.has(typeText) ? COLLECTION_TYPE_COLOR
                    : ENUM_TYPES.has(typeText)        ? ENUM_TYPE_COLOR
                    : RECORD_TYPES.has(typeText)      ? RECORD_TYPE_COLOR
                    : STRUCT_TYPES.has(typeText)      ? STRUCT_TYPE_COLOR
                    : VALUE_TYPES.has(typeText)        ? VALUE_TYPE_COLOR
                    :                                   TYPE_COLOR;
        parent.appendChild(mkTspan(typeText, color));
      } else {
        const root  = typeText.slice(0, lt);
        const inner = typeText.slice(lt + 1, typeText.length - 1); // strip outer < >
        const color = COLLECTION_TYPES.has(root) ? COLLECTION_TYPE_COLOR
                    : ENUM_TYPES.has(root)        ? ENUM_TYPE_COLOR
                    : RECORD_TYPES.has(root)      ? RECORD_TYPE_COLOR
                    : STRUCT_TYPES.has(root)      ? STRUCT_TYPE_COLOR
                    : VALUE_TYPES.has(root)        ? VALUE_TYPE_COLOR
                    :                               TYPE_COLOR;
        parent.appendChild(mkTspan(root, color));
        parent.appendChild(mkTspan('<', GENERIC_COLOR));
        splitTopLevelArgs(inner).forEach((arg, i, arr) => {
          appendType(parent, arg);
          if (i < arr.length - 1) parent.appendChild(mkTspan(', ', GENERIC_COLOR));
        });
        parent.appendChild(mkTspan('>', GENERIC_COLOR));
      }

      if (nullable) parent.appendChild(mkTspan('?', NULLABLE_COLOR));
    }

    // Color the name+params portion: method params get type coloring too.
    function appendRest(parent, rest) {
      const lp = rest.indexOf('(');
      if (lp === -1) {
        // Property — just a name
        parent.appendChild(mkTspan(rest, NAME_COLOR));
        return;
      }
      // Method: color name and '(' together, then each param, then ')'
      parent.appendChild(mkTspan(rest.slice(0, lp + 1), NAME_COLOR));
      const inner = rest.slice(lp + 1, rest.length - 1);
      if (inner) {
        inner.split(', ').forEach((param, i, arr) => {
          const sp = param.indexOf(' ');
          if (sp === -1) {
            parent.appendChild(mkTspan(param, NAME_COLOR));
          } else {
            appendType(parent, param.slice(0, sp));                      // param type
            parent.appendChild(mkTspan('\u00a0' + param.slice(sp + 1), NAME_COLOR)); // param name
          }
          if (i < arr.length - 1) parent.appendChild(mkTspan(', ', null));
        });
      }
      parent.appendChild(mkTspan(')', NAME_COLOR));
    }

    // Adjacency map: node name → array of directly connected node names (bidirectional).
    const ADJACENCY = %s;

    document.querySelectorAll('svg text').forEach(el => {
      const raw = el.textContent;

      // Case 1: standalone visibility symbol rendered by D2 separately.
      // Use tspan (not setAttribute) so we override D2's inline style.fill.
      // D2 only supports +, -, # as vis modifiers; we use '-' for C# "internal"
      // and remap it to '~' for display (private members are excluded from output).
      if (raw.trim() === '+' || raw.trim() === '#' || raw.trim() === '-' || raw.trim() === '@') {
        // Suppress vis symbol when the immediately following sibling text is an enum
        // member row (numeric prefix before U+00A0) — D2 renders + as a separate element.
        const nextEl = el.nextElementSibling;
        if (nextEl && nextEl.tagName.toLowerCase() === 'text') {
          const nb = nextEl.textContent.indexOf('\u00a0');
          if (nb > 0 && /^-?\d+$/.test(nextEl.textContent.substring(0, nb))) {
            while (el.firstChild) el.removeChild(el.firstChild);
            return;
          }
        }
        while (el.firstChild) el.removeChild(el.firstChild);
        const display = raw.trim() === '-' ? '~' : raw.trim();
        el.appendChild(mkTspan(display, VIS_COLOR));
        return;
      }

      // Case 2: enum member row — numeric value + U+00A0 padding + member name.
      // Detected by an all-digit (optionally negative) prefix before the first U+00A0.
      const nbspIdx = raw.indexOf('\u00a0');
      if (nbspIdx <= 0) return;

      let nameStart = nbspIdx;
      while (nameStart < raw.length && raw[nameStart] === '\u00a0') nameStart++;
      const spaces = raw.substring(nbspIdx, nameStart);
      const rest   = raw.substring(nameStart);
      if (!rest) return;

      const prefixText = raw.substring(0, nbspIdx);
      // Strip optional D2 vis symbol (+, -, #) then check for a numeric enum value.
      const enumValMatch = prefixText.match(/^[+\-#]?(-?\d+)$/);
      if (enumValMatch) {
        while (el.firstChild) el.removeChild(el.firstChild);
        el.appendChild(mkTspan(enumValMatch[1], VIS_COLOR));
        el.appendChild(mkTspan(spaces, null));
        el.appendChild(mkTspan(rest, ENUM_TYPE_COLOR));
        return;
      }

      // Case 3: type + U+00A0 padding + name/params
      // In case D2 did NOT strip the vis symbol (future-proofing).
      // '-' is remapped to '~' (UML package-private / C# internal) — private members never appear in output.
      let typeText = prefixText;
      let visPrefix = '';
      if (typeText.length > 1 && (typeText[0] === '+' || typeText[0] === '#' || typeText[0] === '-')) {
        visPrefix = typeText[0] === '-' ? '~' : typeText[0];
        typeText  = typeText.slice(1);
      }

      while (el.firstChild) el.removeChild(el.firstChild);
      if (visPrefix) el.appendChild(mkTspan(visPrefix, VIS_COLOR));
      appendType(el, typeText);
      el.appendChild(mkTspan(spaces, null));
      appendRest(el, rest);
    });

    // ── Zoom / Pan ──────────────────────────────────────────────────────────
    (function() {
      const wrap = document.querySelector('.diagram-wrap');
      const svg  = wrap && wrap.querySelector('svg');
      if (!svg) return;
      let svgW = 0, svgH = 0;
      let scale = 1, tx = 0, ty = 0;
      const MIN = 0.05, MAX = 20;
      function applyXf() {
        svg.style.transform = 'translate('+tx+'px,'+ty+'px) scale('+scale+')';
        const lbl = document.getElementById('zlvl');
        if (lbl) lbl.textContent = Math.round(scale*100)+'%%';
      }
      function fit() {
        if (!svgW || !svgH) return;
        const vw = wrap.clientWidth, vh = wrap.clientHeight;
        if (!vw || !vh) return;
        scale = Math.min(vw/svgW, vh/svgH);
        tx = (vw - svgW*scale) / 2;
        ty = (vh - svgH*scale) / 2;
        applyXf();
      }
      function zoomAt(cx, cy, factor) {
        const ns = Math.min(MAX, Math.max(MIN, scale*factor));
        tx = cx - (cx-tx)*(ns/scale);
        ty = cy - (cy-ty)*(ns/scale);
        scale = ns; applyXf();
      }
      // Measure natural SVG pixel dimensions before any transform is applied,
      // then run the initial fit. getBoundingClientRect() is reliable here because
      // D2 sets explicit width/height on the SVG root and no CSS resizes it yet.
      requestAnimationFrame(function() {
        var bb = svg.getBoundingClientRect();
        svgW = bb.width; svgH = bb.height;
        fit();
      });
      wrap.addEventListener('wheel', function(e) {
        e.preventDefault();
        const r = wrap.getBoundingClientRect();
        zoomAt(e.clientX-r.left, e.clientY-r.top, e.deltaY<0 ? 1.12 : 1/1.12);
      }, { passive: false });
      let drag=false, dx=0, dy=0;
      wrap.addEventListener('mousedown', function(e) {
        if (e.button!==0) return;
        drag=true; dx=e.clientX; dy=e.clientY;
        wrap.classList.add('panning'); e.preventDefault();
      });
      window.addEventListener('mousemove', function(e) {
        if (!drag) return;
        tx+=e.clientX-dx; ty+=e.clientY-dy; dx=e.clientX; dy=e.clientY; applyXf();
      });
      window.addEventListener('mouseup', function() { drag=false; wrap.classList.remove('panning'); });
      let lt=[];
      wrap.addEventListener('touchstart', function(e) { lt=Array.from(e.touches); e.preventDefault(); }, { passive:false });
      wrap.addEventListener('touchmove', function(e) {
        e.preventDefault();
        const t=Array.from(e.touches);
        if (t.length===1 && lt.length===1) {
          tx+=t[0].clientX-lt[0].clientX; ty+=t[0].clientY-lt[0].clientY; applyXf();
        } else if (t.length===2 && lt.length===2) {
          const r=wrap.getBoundingClientRect();
          const d0=Math.hypot(lt[0].clientX-lt[1].clientX, lt[0].clientY-lt[1].clientY);
          const d1=Math.hypot(t[0].clientX-t[1].clientX,  t[0].clientY-t[1].clientY);
          zoomAt((t[0].clientX+t[1].clientX)/2-r.left, (t[0].clientY+t[1].clientY)/2-r.top, d0>0?d1/d0:1);
        }
        lt=t;
      }, { passive:false });
      window.addEventListener('keydown', function(e) {
        if (e.target.tagName==='INPUT') return;
        const r=wrap.getBoundingClientRect(), cx=r.width/2, cy=r.height/2;
        if (e.key==='='||e.key==='+') { zoomAt(cx,cy,1.2); e.preventDefault(); }
        if (e.key==='-')              { zoomAt(cx,cy,1/1.2); e.preventDefault(); }
        if (e.key==='0'||e.key.toLowerCase()==='r') { fit(); e.preventDefault(); }
      });
      var zin=document.getElementById('zin'), zout=document.getElementById('zout'), zrst=document.getElementById('zrst');
      if (zin)  zin.addEventListener('click',  function(){ var r=wrap.getBoundingClientRect(); zoomAt(r.width/2,r.height/2,1.3); });
      if (zout) zout.addEventListener('click', function(){ var r=wrap.getBoundingClientRect(); zoomAt(r.width/2,r.height/2,1/1.3); });
      if (zrst) zrst.addEventListener('click', fit);
    })();

    // ── Hover Highlight ─────────────────────────────────────────────────────
    (function() {
      var outerSvg = document.querySelector('.diagram-wrap svg');
      if (!outerSvg || typeof ADJACENCY === 'undefined') return;

      // D2 wraps its output in a nested SVG. All real elements live inside the inner SVG.
      var root = outerSvg.querySelector('svg') || outerSvg;

      // ── nameToGroup: type name → its <g> element ──────────────────────────
      // For each <g>, inspect its FIRST direct <text> child (the node title).
      // If it matches an ADJACENCY key (strip vis prefix + generic suffix), this is a node group.
      var nameToGroup = new Map();
      root.querySelectorAll('g').forEach(function(g) {
        var titleEl = null;
        for (var i = 0; i < g.children.length; i++) {
          if (g.children[i].tagName.toLowerCase() === 'text') { titleEl = g.children[i]; break; }
        }
        if (!titleEl) return;
        var raw = titleEl.textContent.trim();
        var name = raw.replace(/^[+#\-~]\s*/, '').replace(/<.*$/, '').trim();
        if (Object.prototype.hasOwnProperty.call(ADJACENCY, name) && !nameToGroup.has(name)) {
          nameToGroup.set(name, g);
        }
      });

      // ── edgeGroups: <g> elements containing an arrow path ─────────────────
      // D2 uses marker-end for -> edges (uses) and marker-start for <- edges
      // (extends / implements), so we must select both attribute variants.
      var edgeGroupSet = new Set();
      root.querySelectorAll('path[marker-end], path[marker-start], line[marker-end], line[marker-start]').forEach(function(arrow) {
        var g = arrow.parentElement;
        if (g && g.tagName.toLowerCase() === 'g') edgeGroupSet.add(g);
      });
      var edgeGroups = Array.from(edgeGroupSet);

      // ── containerGroups: project box <g> elements ─────────────────────────
      // D2 renders containers as visual siblings of nodes (NOT DOM parents).
      // All nodes, containers, and edges are direct children of the inner SVG.
      // We identify containers as direct <g> children that are neither node groups
      // nor edge groups, then use bounding-box intersection to map nodes → container.
      var nodeGroupSet = new Set(nameToGroup.values());
      var containerGroups = [];
      var nameToContainer = new Map(); // type name → its container <g> (if any)
      root.querySelectorAll(':scope > g').forEach(function(container) {
        if (nodeGroupSet.has(container) || edgeGroupSet.has(container)) return;
        containerGroups.push(container);
      });
      // Map each node to the container whose bounding box contains its center.
      if (containerGroups.length > 0) {
        nameToGroup.forEach(function(nodeGrp, nodeName) {
          try {
            var nb = nodeGrp.getBBox();
            var cx = nb.x + nb.width / 2, cy = nb.y + nb.height / 2;
            containerGroups.forEach(function(container) {
              try {
                var cb = container.getBBox();
                if (cx >= cb.x && cx <= cb.x + cb.width && cy >= cb.y && cy <= cb.y + cb.height) {
                  nameToContainer.set(nodeName, container);
                }
              } catch(e) {}
            });
          } catch(e) {}
        });
      }
      // Drop containers that contain no nodes — they don't need dimming logic.
      var usedContainers = new Set(nameToContainer.values());
      containerGroups = containerGroups.filter(function(c){ return usedContainers.has(c); });

      // ── edgeGroupNodes: edge <g> → [node names at its endpoints] ──────────
      function getEndpoints(eg) {
        var arrow = eg.querySelector('path[marker-end], path[marker-start], line[marker-end], line[marker-start]');
        if (!arrow) return null;
        if (arrow.tagName.toLowerCase() === 'line') {
          return [
            {x: parseFloat(arrow.getAttribute('x1')), y: parseFloat(arrow.getAttribute('y1'))},
            {x: parseFloat(arrow.getAttribute('x2')), y: parseFloat(arrow.getAttribute('y2'))}
          ];
        }
        var len = arrow.getTotalLength();
        return [arrow.getPointAtLength(0), arrow.getPointAtLength(len)];
      }
      function ptInBBox(pt, grp) {
        try {
          var b = grp.getBBox(), pad = 30;
          return pt.x >= b.x-pad && pt.x <= b.x+b.width+pad && pt.y >= b.y-pad && pt.y <= b.y+b.height+pad;
        } catch(e) { return false; }
      }
      var edgeGroupNodes = new Map();
      edgeGroups.forEach(function(eg) {
        var pts = getEndpoints(eg);
        if (!pts) return;
        var hit = [];
        nameToGroup.forEach(function(grp, name) {
          if (pts.some(function(p){ return ptInBBox(p, grp); })) hit.push(name);
        });
        if (hit.length > 0) edgeGroupNodes.set(eg, hit);
      });

      // ── Hover handlers ─────────────────────────────────────────────────────
      function restoreAll() {
        nameToGroup.forEach(function(g){ g.style.opacity = ''; });
        edgeGroups.forEach(function(g){ g.style.opacity = ''; });
        containerGroups.forEach(function(g){ g.style.opacity = ''; });
      }

      nameToGroup.forEach(function(grp, name) {
        var neighbors = new Set(ADJACENCY[name] || []);
        grp.addEventListener('mouseenter', function() {
          // Which containers hold at least one highlighted node?
          // Those containers stay at full opacity so their highlighted children show through.
          var highlightedContainers = new Set();
          var myC = nameToContainer.get(name);
          if (myC) highlightedContainers.add(myC);
          neighbors.forEach(function(n) { var c = nameToContainer.get(n); if (c) highlightedContainers.add(c); });

          // Dim everything.
          nameToGroup.forEach(function(g){ g.style.opacity = '0.3'; });
          edgeGroups.forEach(function(g){ g.style.opacity = '0.3'; });
          containerGroups.forEach(function(g){
            g.style.opacity = highlightedContainers.has(g) ? '' : '0.3';
          });

          // Restore hovered node + neighbors.
          grp.style.opacity = '1';
          neighbors.forEach(function(n){ var g = nameToGroup.get(n); if (g) g.style.opacity = '1'; });

          // Restore edges connected to the hovered node.
          edgeGroupNodes.forEach(function(nodes, eg){
            if (nodes.indexOf(name) !== -1) eg.style.opacity = '1';
          });
        });
        grp.addEventListener('mouseleave', restoreAll);
      });
    })();
  });
  </script>
  <div class="zoom-bar">
    <button id="zin"  title="Zoom in (=)">+</button>
    <span   id="zlvl">100%%</span>
    <button id="zout" title="Zoom out (-)">&#8722;</button>
    <button id="zrst" title="Reset fit (0)">&#8859;</button>
  </div>
  <div class="legend">
    <div class="legend-section">
      <div class="legend-section-title">Relationships</div>
      <div class="legend-item">
        <svg width="36" height="10" style="flex-shrink:0"><line x1="0" y1="5" x2="36" y2="5" stroke="%s" stroke-width="2" stroke-dasharray="2,3" marker-end="url(#arr-ext)"/><defs><marker id="arr-ext" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto"><path d="M0,0 L6,3 L0,6 Z" fill="%s"/></marker></defs></svg>
        <span>extends</span>
      </div>
      <div class="legend-item">
        <svg width="36" height="10" style="flex-shrink:0"><line x1="0" y1="5" x2="36" y2="5" stroke="%s" stroke-width="2" stroke-dasharray="4,3" marker-end="url(#arr-impl)"/><defs><marker id="arr-impl" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto"><path d="M0,0 L6,3 L0,6 Z" fill="%s"/></marker></defs></svg>
        <span>implements</span>
      </div>
      <div class="legend-item">
        <svg width="36" height="10" style="flex-shrink:0"><line x1="0" y1="5" x2="36" y2="5" stroke="%s" stroke-width="2" marker-end="url(#arr-use)"/><defs><marker id="arr-use" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto"><path d="M0,0 L6,3 L0,6 Z" fill="%s"/></marker></defs></svg>
        <span>uses</span>
      </div>
      <div class="legend-item">
        <span class="legend-ghost" style="background:%s;border-color:%s"></span>
        <span>external type</span>
      </div>
    </div>
    <div class="legend-section">
      <div class="legend-section-title">Visibility</div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s;width:10px;display:inline-block;text-align:center">+</span><span>public</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s;width:10px;display:inline-block;text-align:center">#</span><span>protected</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s;width:10px;display:inline-block;text-align:center">-</span><span>private</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s;width:10px;display:inline-block;text-align:center">~</span><span>internal</span>
      </div>
    </div>
    <div class="legend-section">
      <div class="legend-section-title">Type Colors</div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s">Type</span><span style="color:%s;font-family:%s">&lt;</span><span style="color:%s;font-family:%s">int</span><span style="color:%s;font-family:%s">&gt;</span>&nbsp;<span style="color:%s;font-family:%s">name</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s">List&lt;T&gt;</span>&nbsp;<span>collection</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s">Enum</span>&nbsp;<span>enum type</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s">Record</span>&nbsp;<span>record type</span>
      </div>
      <div class="legend-item">
        <span style="color:%s;font-family:%s">Struct</span>&nbsp;<span>struct type</span>
      </div>
    </div>
  </div>
</body>
</html>
`

// WriteHTML writes a self-contained HTML file embedding the provided SVG,
// then opens it in the system browser.  Returns the path of the written file.
// typeKinds maps type name → kind ("enum", "record", "struct") for JS coloring.
// edges is used to embed a bidirectional adjacency map for hover highlighting.
func WriteHTML(svg []byte, label string, outDir string, typeKinds map[string]string, edges []diagram.Edge) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	title := fmt.Sprintf("PR Diagram — %s", label)
	f := theme.FontFamilyCSS
	adjacencyJSON := buildAdjacencyJSON(edges)
	content := fmt.Sprintf(htmlTemplate,
		// <head>: title, font href
		title,
		theme.FontGoogleHref,
		// CSS: background, text-color, font-family ×2
		theme.Background, theme.MemberName, f, f,
		// SVG body — <div class="diagram-wrap"> comes BEFORE <script>
		string(svg),
		// JS color constants (inside <script> which is after the SVG div)
		theme.Background, theme.MemberVis, theme.MemberType, theme.MemberValueType,
		theme.MemberCollectionType, theme.MemberEnumType, theme.MemberRecordType, theme.MemberStructType,
		theme.MemberGeneric, theme.MemberNullable, theme.MemberName,
		// JS type sets — dynamically generated from parsed source
		buildTypeSets(typeKinds),
		// JS adjacency map for hover highlighting
		adjacencyJSON,
		// legend arrows: extends ×2, implements ×2, uses ×2
		theme.ArrowExtends, theme.ArrowExtends,
		theme.ArrowImplements, theme.ArrowImplements,
		theme.ArrowUses, theme.ArrowUses,
		// legend ghost box: background, border-color
		theme.GhostFill, theme.GhostStroke,
		// legend vis symbols: +, #, ~, @ (color + font each)
		theme.MemberVis, f, theme.MemberVis, f, theme.MemberVis, f, theme.MemberVis, f,
		// legend member color key: Type<int> name, then List, enum, record, struct
		theme.MemberType, f, theme.MemberGeneric, f, theme.MemberValueType, f, theme.MemberGeneric, f, theme.MemberName, f,
		theme.MemberCollectionType, f,
		theme.MemberEnumType, f,
		theme.MemberRecordType, f,
		theme.MemberStructType, f,
	)

	filename := fmt.Sprintf("pr-diagram-%s.html", label)
	filePath := filepath.Join(outDir, filename)

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write HTML: %w", err)
	}

	openBrowser(filePath)
	return filePath, nil
}

// buildTypeSets generates JS const declarations for ENUM_TYPES, RECORD_TYPES,
// and STRUCT_TYPES from the kind map produced by the parser.
func buildTypeSets(typeKinds map[string]string) string {
	sets := map[string][]string{"enum": {}, "record": {}, "struct": {}}
	for name, kind := range typeKinds {
		if _, ok := sets[kind]; ok {
			sets[kind] = append(sets[kind], "'"+name+"'")
		}
	}
	for k := range sets {
		sort.Strings(sets[k])
	}
	return strings.Join([]string{
		"const ENUM_TYPES   = new Set([" + strings.Join(sets["enum"], ",") + "]);",
		"const RECORD_TYPES = new Set([" + strings.Join(sets["record"], ",") + "]);",
		"const STRUCT_TYPES = new Set([" + strings.Join(sets["struct"], ",") + "]);",
	}, "\n    ")
}

// buildAdjacencyJSON serialises a bidirectional adjacency map as a JS object literal.
// Each node name maps to an array of directly connected node names (in either direction).
func buildAdjacencyJSON(edges []diagram.Edge) string {
	adj := make(map[string]map[string]bool)
	for _, e := range edges {
		if adj[e.From] == nil {
			adj[e.From] = make(map[string]bool)
		}
		if adj[e.To] == nil {
			adj[e.To] = make(map[string]bool)
		}
		adj[e.From][e.To] = true
		adj[e.To][e.From] = true
	}
	// Stable output order.
	nodes := make([]string, 0, len(adj))
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	var sb strings.Builder
	sb.WriteString("{")
	for i, node := range nodes {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%q:[", node))
		neighbors := make([]string, 0, len(adj[node]))
		for n := range adj[node] {
			neighbors = append(neighbors, n)
		}
		sort.Strings(neighbors)
		for j, n := range neighbors {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%q", n))
		}
		sb.WriteString("]")
	}
	sb.WriteString("}")
	return sb.String()
}

func openBrowser(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", abs).Start() //nolint:errcheck
	case "darwin":
		exec.Command("open", abs).Start() //nolint:errcheck
	default:
		exec.Command("xdg-open", abs).Start() //nolint:errcheck
	}
}
