package ai

type TextStreamResult struct {
	Stream <-chan string
	Err    <-chan error
}

func NewStreamFromString(text string) *TextStreamResult {
	output := make(chan string)

	go func() {
		output <- text
	}()

	return &TextStreamResult{
		Stream: output,
	}
}

func (t *TextStreamResult) ReadAll() string {
	result := ""
	for next := range t.Stream {
		result += next
	}

	return result
}
