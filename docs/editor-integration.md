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

- A missing glyph or shaped cluster becomes an error-painted placeholder.
- The remainder of the formula keeps its normal geometry and draw order.
- `Box.Diagnostics` and `RenderResult.Diagnostics` identify
  `CodeMissingGlyph` and its source range.
- A host that requires a complete font match can select `RecoveryStrict`.

Recovery happens in layout because glyph coverage depends on the prepared font
set. The canonical IR is never rewritten with font-specific placeholders.

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

## Required Tests

- missing glyphs in atoms, text runs, scripts, fractions, roots, tables, and
  aligned rows;
- strict versus visible recovery;
- cluster fallback for CJK, combining marks, Arabic, Indic, and emoji sequences;
- bidirectional ordering and source cluster partitioning;
- explicit-color preservation and host remapping;
- width-aware wrapping, overfull lines, cache identity, and immutability;
- SVG/PNG fail-closed behavior for any future host-only text item;
- MyGo adapter tests against the supported v0.7.4 dependency.
