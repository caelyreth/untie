package untie

import "github.com/caelyreth/untie/source"

// Code is a stable diagnostic identifier.
type Code = source.Code

const (
	CodeSyntax              = source.CodeSyntax
	CodeUnknownCommand      = source.CodeUnknownCommand
	CodeUnknownSymbol       = source.CodeUnknownSymbol
	CodeMissingArgument     = source.CodeMissingArgument
	CodeUnclosedGroup       = source.CodeUnclosedGroup
	CodeUnexpectedToken     = source.CodeUnexpectedToken
	CodeUnsupported         = source.CodeUnsupported
	CodeInvalidValue        = source.CodeInvalidValue
	CodeInvalidEnvironment  = source.CodeInvalidEnvironment
	CodeUnknownVariable     = source.CodeUnknownVariable
	CodeDuplicateLabel      = source.CodeDuplicateLabel
	CodeUnresolvedReference = source.CodeUnresolvedReference
	CodeMissingGlyph        = source.CodeMissingGlyph
	CodeInsufficientStretch = source.CodeInsufficientStretch
	CodeTextUnsupported     = source.CodeTextUnsupported
	CodeLimitExceeded       = source.CodeLimitExceeded
	CodeCanceled            = source.CodeCanceled
	CodeInternal            = source.CodeInternal
	CodeDeprecated          = source.CodeDeprecated
)
