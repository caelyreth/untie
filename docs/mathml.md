# MathML Interchange

The `mathml` package converts between Untie canonical IR and Presentation
MathML. It is a boundary package: it does not parse TeX or Typst, lay out
glyphs, load fonts, or enter the renderer's cache.

The two directions intentionally have different scopes:

- export is total over every canonical IR node family;
- import accepts a documented Presentation MathML subset and rejects everything
  else with structured diagnostics.

This is not a promise of full MathML or Content MathML support.

## Export

```go
expr := untie.ParseTeX(`x^2+\frac{1}{2}`, untie.Display, untie.ParseOptions{})
formula, err := expr.RootSnapshot()
if err != nil {
	return err
}

document, err := mathml.Marshal(ctx, formula, untie.Display, mathml.Options{})
```

`mathml.Marshal` returns deterministic Presentation MathML. `mathml.Write`
streams the same document to an `io.Writer`; use `Marshal` when atomic output is
required.

The root element is:

```xml
<math xmlns="http://www.w3.org/1998/Math/MathML" display="block">
```

`display="inline"` is used for `ir.Inline`. Export also emits:

- `alttext` and `data-untie-alt-text` from Untie's semantic accessibility text;
- `data-untie-version="1"` for the optional metadata conventions;
- `data-untie-node-id` for assigned stable node IDs;
- `data-untie-class`, sizing, limits, role, spacing, and source-independent
  semantic metadata that MathML has no direct equivalent for;
- `mathcolor`, `mathvariant`, standard spacing attributes, and other ordinary
  Presentation MathML attributes when they accurately represent IR.

Export mappings include:

| Untie IR | Presentation MathML |
| --- | --- |
| row, glyph, text, operator, space | `mrow`, `mi`/`mn`/`mo`, `mtext`, `mo`, `mspace` |
| fraction, radical | `mfrac`, `msqrt`, `mroot` |
| scripts | `msub`, `msup`, `msubsup`, `munder`, `mover`, `munderover`, `mmultiscripts` |
| fenced, sized delimiter | `mrow` with delimiters, `mo` |
| accent, line, cancel | `mover`/`munder`, `menclose` |
| table, multiline | `mtable`, `mtr`, `mtd` |
| style, color, box | `mstyle` |
| phantom | `mphantom` |
| error, placeholder | `merror` |
| break, alignment marker | zero-width `mspace` |

Formula-level equation tags use `mtable`/`mlabeledtr`. Suppression-only tag
events are represented by root metadata because they have no visible MathML
content.

Source spans, origins, and text-cluster maps are deliberately not exported.
They describe a particular TeX or Typst source buffer, not the formula's
language-neutral meaning.

## Import

```go
result, err := mathml.Unmarshal(ctx, document, mathml.Options{})
if err != nil {
	for _, diagnostic := range result.Diagnostics {
		report(diagnostic)
	}
	return err
}
formula := result.Formula
mode := result.Mode
```

`Unmarshal` returns:

- a validated `*ir.Formula` and root `ir.Mode` on success;
- deterministic `source.Diagnostic` values for malformed, limited, or
  unsupported input;
- `ErrInvalidMathML`, `ErrUnsupported`, or `ErrLimitExceeded` as the matching
  error category;
- the context error directly for cancellation or deadline expiry.

Diagnostic ranges are UTF-8 byte ranges into the MathML input. Imported IR
nodes use detached source ranges; the importer never pretends an XML offset is
a TeX or Typst source offset.

The supported element subset is:

`math`, `mi`, `mn`, `mo`, `mtext`, `mspace`, `mrow`, `mfrac`, `msqrt`,
`mroot`, `msub`, `msup`, `msubsup`, `munder`, `mover`, `munderover`, `mtable`,
`mtr`, `mtd`, `mfenced`.

Supported ordinary attributes include `display`, `mathvariant`, `mathcolor`,
`displaystyle`, `scriptlevel`, `lspace`, `rspace`, `linethickness`, `bevelled`,
`stretchy`, `fence`, `separator`, `form`, `largeop`, `accent`, `accentunder`,
`width`, `columnalign`, `rowspacing`, `columnspacing`, `open`, `close`, and
`separators`, each with strict value validation where it affects canonical IR.
Non-semantic document attributes such as `class`, `id`, and `aria-label` are
accepted and ignored. Unknown attributes are rejected.

Optional `data-untie-*` metadata can preserve IR fields that MathML cannot spell
directly, including stable IDs, paint roles, semantic script/table/fraction
roles, and exact nonnegative lengths. MathML without these attributes remains
importable; the importer uses conservative defaults and ordinary operator
classification.

Explicit colors are limited on import to `#rgb`, `#rgba`, `#rrggbb`,
`#rrggbbaa`, and `currentColor`. Named CSS colors and CSS functions are rejected
rather than approximated.

## Deliberate Import Exclusions

The importer rejects these constructs explicitly:

- Content MathML and mixed content markup;
- `semantics`, `annotation`, and other annotation layers;
- `mstyle`, `mphantom`, `menclose`, `merror`, and `mmultiscripts`;
- `mlabeledtr` equation tags;
- links, external references, macros, embedded HTML/SVG, and CSS beyond
  `letter-spacing` on `mtext`;
- unknown elements, attributes, metadata versions, and semantic values;
- DTDs, external entities, processing instructions other than the XML
  declaration, and non-UTF-8 encodings.

Unsupported import is an error, not a silent fallback. Round-tripping the
export is guaranteed for the documented import subset, not for every node kind
that export can represent.

## Validation, Limits, and Safety

Export first validates the full canonical formula with `ir.Validate`. Imported
IR is validated before it is returned. Both directions honor context
cancellation between bounded work units.

`Options.Limits` supplies the canonical IR, XML token, node, nesting, text, and
table limits. `Options.MaxOutputBytes` bounds generated output and defaults to
`mathml.DefaultMaxOutputBytes`.

The XML reader is deterministic and uses Go's standard streaming decoder. It
does not resolve external entities. Input and output are UTF-8, and XML 1.0
invalid control characters are rejected.

Import and export are pure Go and have no font, DOM, browser, network, or system
service dependency.

## Tests and Provenance

The package tests cover:

- deterministic export and XML escaping;
- every canonical IR node kind;
- all supported import elements and semantic metadata;
- explicit diagnostics for unknown and export-only constructs;
- cancellation, traversal limits, input limits, output limits, and malformed
  XML;
- generated-tree round-trip properties and a fuzz property for accepted input;
- TeX cases selected from the reviewed KaTeX fixture corpus under
  `testdata/upstream/katex`.

The upstream fixtures are inputs only. No reference MathML implementation or
reference project is compiled or executed by these tests.
