package markdown

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed test.md
var testmd string

func TestRemoveHiddenText(t *testing.T) {
	//os.WriteFile("tmp.md", []byte(RemoveHiddenText(testmd)), 0644)
	tests := []struct {
		name     string
		markdown string
		expected string
	}{
		{
			name:     "no hidden text",
			markdown: "just some text",
			expected: "just some text",
		},
		{
			name:     "testfile with no malicous input",
			markdown: testmd,
			expected: testmd,
		},
		{
			name:     "valid link",
			markdown: "just some text [stuff](http://mattermost.com) stuff",
			expected: "just some text [stuff](http://mattermost.com) stuff",
		},
		{
			name:     "links empty",
			markdown: "just some text [](hiddentext) stuff",
			expected: "just some text  stuff",
		},
		{
			name:     "links valid but empty",
			markdown: "just some text [   ](https://mattermost.com)",
			expected: "just some text",
		},
		{
			name:     "links valid non-empty",
			markdown: "just some text [MM](https://mattermost.com) and stuff",
			expected: "just some text [MM](https://mattermost.com) and stuff",
		},
		{
			name:     "links not valid non-empty",
			markdown: "just some text [MM](malicoustext) and stuff",
			expected: "just some text  and stuff",
		},
		{
			name:     "image alt text",
			markdown: "just some text ![MM](https://mattermost.com) and stuff",
			expected: "just some text ![](https://mattermost.com) and stuff",
		},
		{
			name:     "image alt text",
			markdown: "just some text ![](somestuff) and stuff",
			expected: "just some text  and stuff",
		},
		{
			name:     "remove reference style links",
			markdown: "just some text [MM][1] and stuff\n[1]: https://mattermost.com",
			expected: "just some text [MM][1] and stuff",
		},
		{
			name:     "remove reference style links more space",
			markdown: "just some text [MM][1] and stuff\n\n[1]: https://mattermost.com",
			expected: "just some text [MM](https://mattermost.com) and stuff",
		},
		{
			name:     "remove extra text in code blocks",
			markdown: "```go and some stuff\nblabla\n```",
			expected: "```go\nblabla\n```",
		},
		{
			name:     "regular code block is fine",
			markdown: "```go\nblabla\n```",
			expected: "```go\nblabla\n```",
		},
		{
			name:     "latex is bad",
			markdown: "```latex\nblabla\n```",
			expected: "",
		},
		{
			name:     "tex is bad",
			markdown: "```tex\nblabla\n```",
			expected: "",
		},
		{
			name:     "inline latex is bad",
			markdown: "this is some stuff $oh no$ bla",
			expected: "this is some stuff  bla",
		},
		{
			name:     "No space after isn't latex. But remove anyway.",
			markdown: "this is some stuff $oh no$bla",
			expected: "this is some stuff bla",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, RemoveHiddenText(tc.markdown))
		})
	}
}
