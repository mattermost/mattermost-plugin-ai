package main

import "image"

type Summarizer interface {
	SummarizeThread(thread string) (string, error)
}

type ThreadAnswerer interface {
	AnswerQuestionOnThread(thread string, question string) (string, error)
}

type ThreadConversationer interface {
	ThreadConversation(thread string, posts []string) (string, error)
}

type EmojiSelector interface {
	SelectEmoji(message string) (string, error)
}

type ImageGenerator interface {
	GenerateImage(prompt string) (image.Image, error)
}
