package ai

import "image"

type Summarizer interface {
	SummarizeThread(thread string) (*TextStreamResult, error)
}

type ThreadAnswerer interface {
	ContinueThreadInterrogation(originalThread string, posts BotConversation) (*TextStreamResult, error)
}

type GenericAnswerer interface {
	ContinueQuestionThread(posts BotConversation) (*TextStreamResult, error)
}

type EmojiSelector interface {
	SelectEmoji(message string) (string, error)
}

type ImageGenerator interface {
	GenerateImage(prompt string) (image.Image, error)
}
