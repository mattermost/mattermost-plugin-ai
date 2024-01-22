package markdown

import (
	"bufio"
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	astext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// RemoveHiddenText removes hidden text from Mattermost markdown for the purpose of
// removing any text that could be prompt injected to an LLM by a malicious user
// without the knowledge of the user mentioning the LLM.
func RemoveHiddenText(markdown string) string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRenderer(newFilteringRenderer()),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(NewInlineLatexParser(), 500),
			),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return ""
	}

	text := FilterText(buf.Bytes())

	return string(bytes.TrimSpace(text))
}

var removeMarkdownReferenceLinks = regexp.MustCompile(`(?m)^[[:space:]]*\[[^\]]+\]\: .*$`)

func FilterText(markdown []byte) []byte {
	// Remove markdown reference style links
	markdown = removeMarkdownReferenceLinks.ReplaceAll(markdown, []byte{})

	return markdown
}

type filteringRenderer struct {
}

func newFilteringRenderer() *filteringRenderer {
	return &filteringRenderer{}
}

func (r *filteringRenderer) Render(output io.Writer, source []byte, n ast.Node) error {
	w := bufio.NewWriter(output)
	ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := node.(type) {
		case *ast.Text:
			if entering {
				segment := n.Segment
				value := segment.Value(source)
				w.Write(value)
				if n.SoftLineBreak() {
					w.WriteByte('\n')
				} else if n.HardLineBreak() {
					w.WriteByte('\n')
				}
			}

		case *ast.String:
			if entering {
				w.Write(n.Value)
			}

		case *ast.Heading:
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
				w.WriteString(strings.Repeat("#", n.Level))
				w.WriteByte(' ')
			}
			if !entering {
				w.WriteByte('\n')
			}

		case *ast.Blockquote:
			if entering {
				w.WriteString("> ")
			}
			if !entering {
				w.WriteByte('\n')
			}

		case *ast.CodeBlock:

		case *ast.FencedCodeBlock:
			language := n.Language(source)
			if bytes.Contains(language, []byte("tex")) || bytes.Contains(language, []byte("latex")) {
				return ast.WalkSkipChildren, nil
			}
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
				w.WriteString("```")
				if language != nil {
					w.Write(language)
				}
				w.WriteByte('\n')
				writeLines(w, source, n)
			}
			if !entering {
				w.WriteString("```\n")
			}

		case *ast.List:

		case *ast.ListItem:
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
				parent, ok := n.Parent().(*ast.List)
				if !ok {
					// This should not happen all list items should be in a list
					return ast.WalkContinue, nil
				}
				if parent.IsOrdered() {
					w.WriteByte('1')
					w.WriteByte(parent.Marker)
					w.WriteByte(' ')
				} else {
					w.WriteByte(parent.Marker)
					w.WriteByte(' ')
				}
			}
			if !entering {
				w.WriteByte('\n')
			}

		case *ast.Paragraph:
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
			}
			if !entering {
				w.WriteByte('\n')
			}

		case *ast.TextBlock:
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
			}
			if !entering {
				if node.NextSibling() != nil && node.FirstChild() != nil {
					w.WriteByte('\n')
				}
			}

		case *ast.ThematicBreak:
			if entering {
				if n.HasBlankPreviousLines() {
					w.WriteByte('\n')
				}
				w.WriteString("---")
			}

		case *ast.AutoLink:
			if entering {
				w.Write(n.URL(source))
			}

		case *ast.Link:
			if entering {
				var text []byte
				if n.HasChildren() {
					if n.ChildCount() != 1 {
						return ast.WalkSkipChildren, nil
					}
					child := n.FirstChild()
					if textNode, ok := child.(*ast.Text); ok {
						text = textNode.Segment.Value(source)
					} else {
						return ast.WalkSkipChildren, nil
					}
				}
				if len(text) == 0 || len(bytes.TrimFunc(text, unicode.IsSpace)) == 0 {
					return ast.WalkSkipChildren, nil
				}
				if n.Destination == nil {
					return ast.WalkSkipChildren, nil
				}
				if destURL, err := url.Parse(string(n.Destination)); err != nil || destURL.Scheme == "" || destURL.Host == "" {
					return ast.WalkSkipChildren, nil
				}
				if bytes.ContainsFunc(n.Destination, unicode.IsSpace) {
					return ast.WalkSkipChildren, nil
				}
				w.WriteByte('[')
				w.Write(text)
				w.WriteByte(']')
				w.WriteByte('(')
				w.Write(n.Destination)
				w.WriteByte(')')
			}
			return ast.WalkSkipChildren, nil

		case *ast.Image:
			if entering {
				if n.Destination == nil || len(bytes.TrimFunc(n.Destination, unicode.IsSpace)) == 0 {
					return ast.WalkSkipChildren, nil
				}
				if bytes.ContainsFunc(n.Destination, unicode.IsSpace) {
					return ast.WalkSkipChildren, nil
				}
				if destURL, err := url.Parse(string(n.Destination)); err != nil || destURL.Scheme == "" || destURL.Host == "" {
					return ast.WalkSkipChildren, nil
				}
				if bytes.ContainsFunc(n.Destination, unicode.IsSpace) {
					return ast.WalkSkipChildren, nil
				}
				w.WriteString("![](")
				w.Write(n.Destination)
				w.WriteByte(')')
			}
			return ast.WalkSkipChildren, nil

		case *ast.CodeSpan:
			w.WriteByte('`')

		case *ast.Emphasis:
			w.WriteString(strings.Repeat("*", n.Level))

		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil

		case *astext.Strikethrough:
			w.WriteString("~~")

		case *InlineLatex:
			return ast.WalkSkipChildren, nil

		}
		return ast.WalkContinue, nil
	})
	return w.Flush()
}

func (r *filteringRenderer) AddOptions(opts ...renderer.Option) {
}

func writeLines(w util.BufWriter, source []byte, n ast.Node) {
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		value := line.Value(source)
		_, _ = w.Write(value)
	}
}
