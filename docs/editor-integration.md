# Editor Integration

An editor stresses different guarantees than a static Markdown preview. A
formula is frequently incomplete, the host owns colors and selection, and the
source-to-geometry mapping must remain useful while the user types.

## Ownership

Untie owns:

- TeX and Typst parsing;
- canonical, font-independent IR;
- OpenType MATH glyph choice, variants, assemblies, and spacing;
- formula layout and the immutable display list;
- deterministic pure-Go text shaping when no host shaper is configured;
- source ranges and diagnostics.

The host owns:

- caret and selection gestures;
- choosing whether a formula displays rendered or as source;
- theme colors and contrast policy;
- system font discovery and registration;
- optional platform text shaping for text-mode runs;
- platform focus, input methods, and accessibility integration.

The MyGo adapter is a bridge, not a second owner of Untie's math font state.

## Visible Failure

The default layout recovery mode is `RecoveryVisible`.

- A missing glyph or shaped cluster becomes an error-painted placeholder with
  `CodeMissingGlyph`.
- An unsupported layout construct becomes a placeholder with
  `CodeUnsupported`.
- A delimiter or accent that exceeds the font's stretch data becomes a
  placeholder with `CodeInsufficientStretch`.
- The remainder of the formula keeps its normal geometry and draw order.
- `Box.Diagnostics` and `RenderResult.Diagnostics` identify
  the recoverable problem and its source range.
- A host that requires a complete font match can select `RecoveryStrict`.

Recovery happens in layout because glyph coverage depends on the prepared font
set. The canonical IR is never rewritten with font-specific placeholders.
Empty source produces a stable placeholder-sized root with a clickable hit.

Cancellation, resource limits, malformed IR, and internal invariant failures
remain hard errors. Visible recovery must not hide work that did not complete
or output that violates the display contract.

## Text And Math Are Different

Math symbols and general text must not share one shaping policy.

Math glyphs use OpenType MATH tables and Untie's deterministic `ssty`/`dtls`
and assembly handling. Text in `\text{}`, embedded strings, and comparable
text-mode runs needs normal text shaping:

- extended grapheme clusters;
- ligatures, kerning, and mark positioning;
- script and language selection;
- bidirectional segmentation;
- per-cluster fallback;
- source cluster ranges.

The default implementation must remain pure Go, deterministic, and independent
of system locale and font discovery.

Parsed `ir.Text` nodes carry `ir.TextClusterSource` entries that map decoded
text bytes back to absolute source bytes. Layout emits one `HitAtom` per shaped
visual cluster (coalescing multiple shaped clusters that share one macro-origin
source range). Bidirectional runs therefore expose hits in visual order while
the canonical IR map remains ordered in logical text order. Text without a
parser-authored map keeps the existing whole-node hit fallback.

`Box.FontUsage` separately reports the resolved face and fallback state for
text clusters in source order. Hosts can detect an already-used fallback with
`UsesFallback()` and collect missing coverage requirements with
`NeedsFallback()` without scraping diagnostics. The report merges adjacent
equivalent spans and remains UI- and font-loader-independent.

The 0.4 display contract is outline-only. Monochrome outline emoji can be
shaped and drawn; fonts that expose only bitmap or COLR color emoji are outside
the current font loader and display-item model. An unsupported color cluster
must use the same visible missing-cluster placeholder and diagnostic rather
than disappearing.

## Caret Stops and Source Identity

`Box.Carets()` returns source-ordered caret stops in parent-frame coordinates.
Every stop contains the absolute source byte offset, horizontal position,
vertical top and bottom, and an optional `NodeID`.

`Box.CaretAt(offset)` prefers an exact proven boundary and otherwise returns the
nearest stop. Shaped text exposes cluster boundaries rather than inventing
interior positions for a ligature or grapheme cluster. RTL clusters map source
start and end to the correct visual edges, and bidi mixed text remains sorted by
source even when hits are stored in visual paint order.

Use `NodeID` when the host needs stable animation or selection identity:

- assign a nonzero `ir.NodeID` to canonical nodes;
- keep IDs unique within one formula;
- preserve an ID yourself when rebuilding a tree after a source edit;
- read the same ID from emitted `Hit` and `Item` records.

Untie deliberately provides no automatic identity inference and no `Expr.Edit`.
Without an incremental parser and a source-to-syntax correspondence, automatic
stable IDs would be offset-derived guesses. The explicit host assignment rule
is the smallest sound boundary until that infrastructure exists.

## Inline Splitting

Editors that break paragraph text inside an inline formula can call
`box.Split(box.Breaks[i])`. The returned boxes contain independent items,
paths, clipped hits, caret stops, and remaining break opportunities. The left
box keeps parent coordinates; the right box is translated to a zero origin.

Only untagged inline boxes are splittable. Width-constrained display boxes use
`Style.MaxWidth` and `Box.Lines`; equation-tagged boxes retain their single
unambiguous tag owner. Alternative text is cleared on split fragments because
there is no sound semantic split for `Alt`.

## Hybrid Text-Shaper Boundary

The long-term architecture is a hybrid:

- Untie's pure-Go shaper is the canonical default for SVG, PNG, WASM, and
  deterministic tests.
- A framework-neutral host text-shaper port may replace shaping for text-mode
  runs only.
- MyGo may implement that port through its platform text engine.
- A host-shaped run must return the exact metrics used for layout before draw;
  guessed metrics followed by different draw metrics are not acceptable.

A host-shaped text item must be a descriptor, not a UI pointer or callback in
the display list. Generic SVG and PNG encoders must fail clearly when asked to
serialize a host run they cannot outline.

## MyGo Font System

MyGo's public registry supplies platform text families, shaping, fallback, and
color-glyph drawing. It does not expose the complete MATH font data or MATH
tables required by Untie.

Therefore:

- Untie packages keep their own prepared `*untie.FontSet`;
- registering that font with MyGo does not replace Untie font ownership;
- MyGo-native text shaping, when added, applies only to `ir.Text`;
- the host-shaper identity, platform text generation, language, direction, and
  registered-font generation must participate in layout cache identity;
- MyGo-native text is explicitly non-portable and cannot be silently omitted
  from SVG or PNG output.

Until MyGo exposes a public text-environment generation and per-cluster glyph
geometry, MyGo-native shaping can provide correct drawing but source mapping
inside `\text{}` may be approximate. The deterministic Untie shaper remains the
default.

## Color Policy

`untie.Paint` carries both semantic role and explicit source color. The default
policy is:

- semantic `Ink`, `Error`, and `Placeholder` roles follow the host theme;
- explicit `\color`, `\textcolor`, or Typst color values remain authoritative.

A host may supply a draw-time paint resolver. This is the intended place for
dark-mode adaptation, contrast checks, or application palettes. Untie does not
silently invert authored colors because color can carry mathematical meaning.

`Style.ColorPolicy` is the core, serializable alternative when the resolved
policy should participate in layout output and cache identity:

- `ColorPreserve` is the zero value and leaves explicit colors unchanged;
- `ColorMapToRoles` maps each explicit color to the perceptually nearest
  `Ink`, `Error`, or `Placeholder` in a host-supplied `ColorRolePalette`;
- `ColorAdjustContrast` minimally adjusts only explicit RGB colors until they
  meet `MinContrast` against an opaque `Background`, preserving alpha when the
  threshold is achievable.

Semantic role paints are never rewritten by a color policy. The output remains
ordinary `display.Paint` data, so it serializes, caches, and flows through the
existing MyGo `PaintResolver` without callbacks or UI types in the display
list.

The MyGo adapter exposes `PaintResolver` and `DrawBox`; the core display list
stays unchanged across light and dark themes.

## Width-Aware Display Math

Editors know the available frame width and should lay out for it before drawing.

- Pass the frame as `Style.MaxWidth`.
- Read line breaks from `Box.Lines`.
- Draw at natural scale for an editor.
- Use uniform fitting only for compact previews where the final width is
  unknowable during construction.

The MyGo adapter exposes this distinction as `DisplayMathWidth` versus
`DisplayMath`.

## MyGo Editor Surface

The MyGo adapter exposes editor-facing contracts without depending on private
MyGo internals:

- `ElementWithOptions` and the width-aware variants attach optional hit,
  diagnostic, and clipboard behavior to one element;
- `OnHit` reports hover, press, and click phases with the `untie.Hit` resolved
  by `Box.At`;
- `HitAt` exposes the inverse draw transform for custom event dispatch;
- `DiagnosticMarks` maps diagnostics through `Box.Covering`, while
  `DiagnosticOptions.Paint` and `Format` are clean drawing and localization
  hooks;
- `FormulaClipboardData` and the context-menu option provide plain text and
  dialect-specific source without inventing a MathML representation;
- `InlineIgnoreOverflow` reports nominal inline baseline metrics while keeping
  certified overflow drawable through the host text clip.

MyGo v0.7.4 does not expose a reliable pointer-leave event or per-inline-object
accessibility metadata. Hover state must therefore be cleared by the host when
another hit begins, and Markdown inline math cannot independently carry spoken
alternative text through the pinned `markdown.InlineObject` contract.

## Required Tests

- missing glyphs in atoms, text runs, scripts, fractions, roots, tables, and
  aligned rows;
- strict versus visible recovery;
- cluster fallback for CJK, combining marks, Arabic, Indic, and emoji sequences;
- bidirectional ordering and source cluster partitioning;
- explicit-color preservation and host remapping;
- width-aware wrapping, overfull lines, cache identity, and immutability;
- accessibility label fallback and source value construction;
- hit testing under `ScaleFit`, `ScaleNatural`, offsets, and pointer phases;
- diagnostic range mapping, custom painters, formatting, and tooltips;
- clipboard source/plain formats and context-menu behavior;
- include-overflow versus nominal inline metrics and baseline placement;
- SVG/PNG fail-closed behavior for any future host-only text item;
- MyGo adapter tests against the supported v0.7.4 dependency.
