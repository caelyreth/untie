package untiemygo

import (
	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const (
	// FormatTeX is the source representation used for TeX formulas.
	FormatTeX transfer.Format = "text/x-tex"
	// FormatTypst is the source representation used for Typst formulas.
	FormatTypst transfer.Format = "text/x-typst"
)

// ClipboardOptions configure the formula context menu.
type ClipboardOptions struct {
	// Enabled adds explicit Copy Plain Text and Copy Source actions.
	Enabled bool
}

// SourceFormat returns the source MIME format for dialect.
func SourceFormat(dialect untie.Dialect) (transfer.Format, bool) {
	switch dialect {
	case untie.TeX:
		return FormatTeX, true
	case untie.Typst:
		return FormatTypst, true
	default:
		return "", false
	}
}

// PlainTextClipboardData returns a clipboard payload containing only text.
func PlainTextClipboardData(text string) transfer.Data {
	return transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(text))))
}

// SourceClipboardData returns a clipboard payload containing formula source
// in the dialect-specific source format.
func SourceClipboardData(source string, dialect untie.Dialect) (transfer.Data, bool) {
	format, ok := SourceFormat(dialect)
	if !ok {
		return transfer.Data{}, false
	}
	return transfer.New(transfer.NewItem(
		transfer.Bytes(format, []byte(source)),
	)), true
}

// FormulaClipboardData returns one clipboard item with plain text and the
// dialect-specific source representation.
//
// The plain representation prefers alt.Text and falls back to source. MathML
// is intentionally absent until Untie has a real serializer.
func FormulaClipboardData(
	source string,
	alt untie.Alt,
	dialect untie.Dialect,
) (transfer.Data, bool) {
	format, ok := SourceFormat(dialect)
	if !ok {
		return transfer.Data{}, false
	}
	plain := DescribeFormula(source, alt).PlainText
	return transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(plain)),
		transfer.Bytes(format, []byte(source)),
	)), true
}

func addClipboardMenu(
	element ui.Element,
	c *ui.Context,
	source string,
	alt untie.Alt,
	dialect untie.Dialect,
) ui.Element {
	if !element.Valid() || c == nil {
		return element
	}
	semantics := DescribeFormula(source, alt)
	sourceLabel := "Copy TeX Source"
	if dialect == untie.Typst {
		sourceLabel = "Copy Typst Source"
	}
	return element.ContextMenu(func(menu *ui.Menu) {
		if menu.Item("Copy Plain Text").Chosen() {
			c.Services().WriteClipboardData(
				PlainTextClipboardData(semantics.PlainText),
			)
		}
		if menu.Item(sourceLabel).Chosen() {
			if data, ok := SourceClipboardData(source, dialect); ok {
				c.Services().WriteClipboardData(data)
			}
		}
	})
}
