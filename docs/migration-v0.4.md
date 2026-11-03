# Migrating To 0.4

Untie 0.4 turns the MyGo adapter's editor integrations into supported public
contracts. Core parsing, canonical IR, and display-list geometry retain the
0.3 model.

## Semantic Accessibility

Standalone MyGo elements now request semantic alternative text and expose:

- `Alt.Speech` as the label, with `Alt.Text` and source as fallbacks;
- exact TeX or Typst source as the value;
- a stable non-empty name for empty formulas.

Set `Options.Accessibility = untiemygo.AccessibilityDisabled` to retain
source-only labels without the alternative-text traversal. Use
`DescribeFormula` when an application needs the same deterministic fallback
policy outside an element.

MyGo v0.7.4's `markdown.InlineObject` has no per-object accessibility metadata.
Inline formulas continue to participate in the paragraph's source-copy and
selection behavior, but cannot independently attach spoken alternative text
through the pinned adapter API.

## Editor Elements

The new options-bearing element methods add:

- `OnHit` hover, press, and click callbacks resolved through `Box.At`;
- source-range diagnostic underlines and whole-element tooltips;
- plain-text and dialect-source clipboard context menu actions.

`HitAt` exposes the inverse draw transform for applications that do not use
MyGo input callbacks.

## Inline Metrics

`Options.Inline.Metrics` selects:

- `InlineIncludeOverflow`, the unchanged default; or
- `InlineIgnoreOverflow` for nominal baseline metrics with overflow ink kept
  drawable through the surrounding text clip.

## Clipboard

`FormulaClipboardData` and the context-menu option expose plain text and custom
TeX/Typst source MIME formats. MathML is not offered until Untie has a real
MathML serializer.

## Version And Cache

`untie.Version()` is now `0.4.0+layout.2`. Layout behavior is unchanged from
0.3, so the layout revision and Engine cache schema remain stable.
