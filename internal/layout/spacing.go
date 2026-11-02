package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

type classGroup uint8

const (
	groupOrd classGroup = iota
	groupOp
	groupBin
	groupRel
	groupOpen
	groupClose
	groupPunct
	groupInner
	groupNone
	groupCount
)

type spaceCode uint8

const (
	spaceNone spaceCode = iota
	spaceThin
	spaceMedium
	spaceThick
)

// TeXbook's eight-by-eight inter-atom spacing table. Invalid binary positions
// are represented by spaceNone and are additionally normalized by
// NormalizeClass before row assembly.
var classSpacing = [groupCount][groupCount]spaceCode{
	groupOrd:   {groupOrd: spaceNone, groupOp: spaceThin, groupBin: spaceMedium, groupRel: spaceThick, groupOpen: spaceNone, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceThin, groupNone: spaceNone},
	groupOp:    {groupOrd: spaceThin, groupOp: spaceThin, groupBin: spaceNone, groupRel: spaceThick, groupOpen: spaceNone, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceThin, groupNone: spaceNone},
	groupBin:   {groupOrd: spaceMedium, groupOp: spaceMedium, groupBin: spaceNone, groupRel: spaceNone, groupOpen: spaceMedium, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceMedium, groupNone: spaceNone},
	groupRel:   {groupOrd: spaceThick, groupOp: spaceThick, groupBin: spaceNone, groupRel: spaceNone, groupOpen: spaceThick, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceThick, groupNone: spaceNone},
	groupOpen:  {groupOrd: spaceNone, groupOp: spaceNone, groupBin: spaceNone, groupRel: spaceNone, groupOpen: spaceNone, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceNone, groupNone: spaceNone},
	groupClose: {groupOrd: spaceNone, groupOp: spaceThin, groupBin: spaceMedium, groupRel: spaceThick, groupOpen: spaceNone, groupClose: spaceNone, groupPunct: spaceNone, groupInner: spaceThin, groupNone: spaceNone},
	groupPunct: {groupOrd: spaceThin, groupOp: spaceThin, groupBin: spaceNone, groupRel: spaceThin, groupOpen: spaceThin, groupClose: spaceThin, groupPunct: spaceThin, groupInner: spaceThin, groupNone: spaceNone},
	groupInner: {groupOrd: spaceThin, groupOp: spaceThin, groupBin: spaceMedium, groupRel: spaceThick, groupOpen: spaceThin, groupClose: spaceNone, groupPunct: spaceThin, groupInner: spaceThin, groupNone: spaceNone},
}

// NormalizeClass resolves context-sensitive atom classes the way TeX does:
// a varying operator becomes binary after an operand, while a binary operator
// in an operator position becomes ordinary.
func NormalizeClass(previous *ir.MathClass, current ir.MathClass) ir.MathClass {
	if current == ir.ClassVarying {
		if previous != nil {
			switch *previous {
			case ir.ClassNormal, ir.ClassAlphabetic, ir.ClassClosing, ir.ClassFence:
				return ir.ClassBinary
			}
		}
		return ir.ClassUnary
	}
	if current != ir.ClassBinary {
		return current
	}
	if previous == nil {
		return ir.ClassUnary
	}
	switch *previous {
	case ir.ClassBinary, ir.ClassRelation, ir.ClassOpening, ir.ClassPunctuation, ir.ClassLarge:
		return ir.ClassUnary
	}
	return current
}

// normalizeRowClasses performs TeX's full atom-class normalization. Binary
// operators are demoted when either side cannot accept a binary atom, which
// requires one step of lookahead rather than only the previous class.
func normalizeRowClasses(classes []ir.MathClass) []ir.MathClass {
	out := make([]ir.MathClass, len(classes))
	var previous *ir.MathClass
	for i, current := range classes {
		next := nextAtomClass(classes, i+1)
		normalized := normalizeAtomClass(previous, current, next)
		out[i] = normalized
		if classOf(normalized) != groupNone {
			value := normalized
			previous = &value
		}
	}
	return out
}

func nextAtomClass(classes []ir.MathClass, start int) *ir.MathClass {
	for i := start; i < len(classes); i++ {
		if classOf(classes[i]) == groupNone {
			continue
		}
		value := classes[i]
		return &value
	}
	return nil
}

func normalizeAtomClass(previous *ir.MathClass, current ir.MathClass, next *ir.MathClass) ir.MathClass {
	if current == ir.ClassVarying {
		if previous != nil {
			switch *previous {
			case ir.ClassNormal, ir.ClassAlphabetic, ir.ClassClosing, ir.ClassFence, ir.ClassUnary:
				current = ir.ClassBinary
			default:
				return ir.ClassUnary
			}
		} else {
			return ir.ClassUnary
		}
	}
	if current != ir.ClassBinary {
		return current
	}
	if !binaryLeftAllowed(previous) || !binaryRightAllowed(next) {
		return ir.ClassUnary
	}
	return current
}

func binaryLeftAllowed(previous *ir.MathClass) bool {
	if previous == nil {
		return false
	}
	switch *previous {
	case ir.ClassBinary, ir.ClassRelation, ir.ClassOpening, ir.ClassPunctuation, ir.ClassLarge:
		return false
	default:
		return true
	}
}

func binaryRightAllowed(next *ir.MathClass) bool {
	if next == nil {
		return false
	}
	switch *next {
	case ir.ClassRelation, ir.ClassClosing, ir.ClassFence, ir.ClassPunctuation:
		return false
	default:
		return true
	}
}

// MathSpacing returns the automatic neutral inter-atom space in local ems.
// Script and script-script styles suppress all automatic spacing. Cramped
// affects constructions, not inter-atom spacing.
func MathSpacing(left, right ir.MathClass, style Style) geometry.Em {
	if style.Size == Script || style.Size == ScriptScript {
		return 0
	}
	lg, rg := classOf(left), classOf(right)
	if lg == groupNone || rg == groupNone {
		return 0
	}
	switch classSpacing[lg][rg] {
	case spaceThin:
		return ThinSpace
	case spaceMedium:
		return MediumSpace
	case spaceThick:
		return ThickSpace
	default:
		return 0
	}
}

func classOf(class ir.MathClass) classGroup {
	switch class {
	case ir.ClassNormal, ir.ClassAlphabetic, ir.ClassUnary, ir.ClassDiacritic:
		return groupOrd
	case ir.ClassBinary, ir.ClassVarying:
		return groupBin
	case ir.ClassRelation:
		return groupRel
	case ir.ClassOpening:
		return groupOpen
	case ir.ClassClosing, ir.ClassFence:
		return groupClose
	case ir.ClassPunctuation:
		return groupPunct
	case ir.ClassLarge:
		return groupOp
	case ir.ClassInner:
		return groupInner
	case ir.ClassSpace, ir.ClassSpecial:
		return groupNone
	default:
		return groupNone
	}
}
