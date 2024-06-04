package ai

type EmbeddingModel interface {
	Embed(text string) ([]float32, error)
}
