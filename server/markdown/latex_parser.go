package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type dollarsignDelimiterProcessor struct {
}

func (p *dollarsignDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '$'
}

func (p *dollarsignDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (p *dollarsignDelimiterProcessor) OnMatch(consumes int) ast.Node {
	return NewInlineLatex()
}

type inlineLatexParser struct {
}

func NewInlineLatexParser() parser.InlineParser {
	return &inlineLatexParser{}
}

func (s *inlineLatexParser) Trigger() []byte {
	return []byte{'$'}
}

func (s *inlineLatexParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 1, &dollarsignDelimiterProcessor{})
	if node == nil {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (s *inlineLatexParser) CloseBlock(parent ast.Node, pc parser.Context) {
}

type InlineLatex struct {
	ast.BaseInline
}

func (n *InlineLatex) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

var KindInlineLatex = ast.NewNodeKind("InlineLatex")

func (n *InlineLatex) Kind() ast.NodeKind {
	return KindInlineLatex
}

func NewInlineLatex() *InlineLatex {
	return &InlineLatex{}
}
