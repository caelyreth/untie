package fixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/source"
)

const maxCSTDepth = 2048

// CSTNode is one node in Typlet's compact Typst concrete syntax tree. Its
// offsets are UTF-16 code units; ResolveCST converts them to UTF-8 byte ranges.
type CSTNode struct {
	Kind     string
	Span     UTF16Range
	Message  string
	Hints    []string
	Children []CSTNode
}

// UnmarshalJSON decodes Typlet's compact node arrays:
//
//	[kind, start, end]
//	[kind, start, end, children]
//	["Error", start, end, message, hints]
func (n *CSTNode) UnmarshalJSON(data []byte) error {
	decoded, err := decodeCSTNode(data, 1)
	if err != nil {
		return err
	}
	if err := decoded.Validate(); err != nil {
		return err
	}
	*n = decoded
	return nil
}

// MarshalJSON restores a deterministic compact node array.
func (n CSTNode) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, err
	}
	if n.Kind == "Error" {
		hints := n.Hints
		if hints == nil {
			hints = []string{}
		}
		return json.Marshal([]any{n.Kind, n.Span.Start, n.Span.End, n.Message, hints})
	}
	if len(n.Children) == 0 {
		return json.Marshal([]any{n.Kind, n.Span.Start, n.Span.End})
	}
	return json.Marshal([]any{n.Kind, n.Span.Start, n.Span.End, n.Children})
}

// Validate checks the compact tree's structural and UTF-16 invariants.
func (n *CSTNode) Validate() error {
	if n == nil {
		return errors.New("nil CST node")
	}
	return validateCSTNode(n, 1, nil)
}

func decodeCSTNode(data []byte, depth int) (CSTNode, error) {
	if depth > maxCSTDepth {
		return CSTNode{}, fmt.Errorf("CST nesting exceeds %d levels", maxCSTDepth)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return CSTNode{}, fmt.Errorf("CST node: %w", err)
	}
	if len(values) != 3 && len(values) != 4 && len(values) != 5 {
		return CSTNode{}, fmt.Errorf("CST node has %d fields; want 3, 4, or 5", len(values))
	}
	var kind string
	if err := json.Unmarshal(values[0], &kind); err != nil {
		return CSTNode{}, fmt.Errorf("CST kind: %w", err)
	}
	start, err := parseInt(values[1], "CST start")
	if err != nil {
		return CSTNode{}, err
	}
	end, err := parseInt(values[2], "CST end")
	if err != nil {
		return CSTNode{}, err
	}
	span := UTF16Range{Start: start, End: end}
	if err := span.Validate(); err != nil {
		return CSTNode{}, err
	}

	decoded := CSTNode{Kind: kind, Span: span}
	if kind == "Error" {
		if len(values) != 5 {
			return CSTNode{}, fmt.Errorf("error CST node has %d fields; want 5", len(values))
		}
		if err := json.Unmarshal(values[3], &decoded.Message); err != nil {
			return CSTNode{}, fmt.Errorf("CST error message: %w", err)
		}
		var hints []string
		if err := json.Unmarshal(values[4], &hints); err != nil {
			return CSTNode{}, fmt.Errorf("CST error hints: %w", err)
		}
		decoded.Hints = append([]string(nil), hints...)
	} else {
		if len(values) == 5 {
			return CSTNode{}, errors.New("only Error CST nodes may have five fields")
		}
		if len(values) == 4 {
			var children []json.RawMessage
			if err := json.Unmarshal(values[3], &children); err != nil {
				return CSTNode{}, fmt.Errorf("CST children: %w", err)
			}
			decoded.Children = make([]CSTNode, len(children))
			for i, child := range children {
				node, err := decodeCSTNode(child, depth+1)
				if err != nil {
					return CSTNode{}, fmt.Errorf("CST child %d: %w", i, err)
				}
				decoded.Children[i] = node
			}
		}
	}
	return decoded, nil
}

func validateCSTNode(node *CSTNode, depth int, parent *UTF16Range) error {
	if depth > maxCSTDepth {
		return fmt.Errorf("CST nesting exceeds %d levels", maxCSTDepth)
	}
	if node == nil {
		return errors.New("nil CST node")
	}
	if node.Kind == "" {
		return errors.New("CST node has an empty kind")
	}
	if err := node.Span.Validate(); err != nil {
		return fmt.Errorf("CST node %s: %w", node.Kind, err)
	}
	if parent != nil && (node.Span.Start < parent.Start || node.Span.End > parent.End) {
		return fmt.Errorf("CST node %s span [%d,%d) is outside parent [%d,%d)",
			node.Kind, node.Span.Start, node.Span.End, parent.Start, parent.End)
	}
	if node.Kind == "Error" {
		if node.Message == "" {
			return errors.New("error CST node has an empty message")
		}
		if len(node.Children) != 0 {
			return errors.New("error CST node cannot have children")
		}
		return nil
	}
	if node.Message != "" || len(node.Hints) != 0 {
		return fmt.Errorf("CST node %s carries error-only fields", node.Kind)
	}
	for i := range node.Children {
		if err := validateCSTNode(&node.Children[i], depth+1, &node.Span); err != nil {
			return fmt.Errorf("CST child %d: %w", i, err)
		}
	}
	return nil
}

// SourceCSTNode is a CST node with a validated UTF-8 byte range.
type SourceCSTNode struct {
	Kind     string
	Span     source.Range
	Message  string
	Hints    []string
	Children []SourceCSTNode
}

// ResolveCST converts every node offset in the UTF-16 CST to a UTF-8 byte range.
func ResolveCST(src string, root *CSTNode) (*SourceCSTNode, error) {
	if err := root.Validate(); err != nil {
		return nil, err
	}
	index, err := newUTF16Index(src)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveCSTNode(index, root, 1, nil)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func resolveCSTNode(index *utf16Index, node *CSTNode, depth int, parent *source.Range) (SourceCSTNode, error) {
	if depth > maxCSTDepth {
		return SourceCSTNode{}, fmt.Errorf("CST nesting exceeds %d levels", maxCSTDepth)
	}
	span, err := index.byteRange(node.Span.Start, node.Span.End)
	if err != nil {
		return SourceCSTNode{}, fmt.Errorf("CST node %s: %w", node.Kind, err)
	}
	if parent != nil && !parent.ContainsRange(span) {
		return SourceCSTNode{}, fmt.Errorf("CST node %s byte range %#v is outside parent %#v", node.Kind, span, *parent)
	}
	resolved := SourceCSTNode{
		Kind: node.Kind, Span: span, Message: node.Message,
		Hints: append([]string(nil), node.Hints...),
	}
	if len(node.Children) > 0 {
		resolved.Children = make([]SourceCSTNode, len(node.Children))
		for i := range node.Children {
			child, err := resolveCSTNode(index, &node.Children[i], depth+1, &span)
			if err != nil {
				return SourceCSTNode{}, fmt.Errorf("CST child %d: %w", i, err)
			}
			resolved.Children[i] = child
		}
	}
	return resolved, nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...)
}
