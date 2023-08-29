package subtitles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSubtitles = `WEBVTT

1
00:00:00.000 --> 00:00:05.600
But just with a variety of reasons, what I have is a pull request. And so I'd like to

2
00:00:06.320 --> 00:00:09.840
simultaneously go back and just, you know, solicit that feedback in case there's some

3
00:00:09.840 --> 00:00:14.560
blind spots here. Obviously, if there are, we need to fix them. That's great. But also to,

4
00:00:15.600 --> 00:00:20.480
if there isn't, but also to communicate some of the changes happening around prepackaged plugins.
`

const expectedFormatForLLM = `00:00 to 00:06 - But just with a variety of reasons, what I have is a pull request. And so I'd like to
00:06 to 00:10 - simultaneously go back and just, you know, solicit that feedback in case there's some
00:10 to 00:15 - blind spots here. Obviously, if there are, we need to fix them. That's great. But also to,
00:16 to 00:20 - if there isn't, but also to communicate some of the changes happening around prepackaged plugins.`

const expectedFormatTextOnly = `But just with a variety of reasons, what I have is a pull request. And so I'd like to simultaneously go back and just, you know, solicit that feedback in case there's some blind spots here. Obviously, if there are, we need to fix them. That's great. But also to, if there isn't, but also to communicate some of the changes happening around prepackaged plugins.`

func TestFormatForLLM(t *testing.T) {
	subtitles, err := NewSubtitlesFromVTT(strings.NewReader(testSubtitles))
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expectedFormatForLLM, subtitles.FormatForLLM())
}

func TestFormatTextOnly(t *testing.T) {
	subtitles, err := NewSubtitlesFromVTT(strings.NewReader(testSubtitles))
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expectedFormatTextOnly, subtitles.FormatTextOnly())
}
