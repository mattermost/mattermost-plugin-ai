package main

/*
import (
	"fmt"

	llama "github.com/go-skynet/go-llama.cpp"
)

const (
	modelFile = "/home/christopher/ml/llama.cpp/mmodels/alpaca/ggml-model-q4_1.bin"
	ctxSize   = 2048
)

func InitLlama() (*llama.LLama, error) {
	fmt.Println("Loading model")
	llm, err := llama.New(modelFile, llama.SetContext(ctxSize), llama.SetParts(-1), llama.EnableF16Memory)
	if err != nil {
		return nil, err
	}
	fmt.Println("Model load compelete")

	return llm, nil
}

func LamaPredict(llm *llama.LLama, message string) string {

	lstart := NewTimer("Predicting...")
	params := []llama.PredictOption{
		llama.SetTokens(0),
		llama.SetThreads(7),
		llama.SetTopK(10000),
		llama.SetTopP(0.9),
		llama.SetBatch(256),
		llama.SetTemperature(0.2),
		llama.SetPenalty(1.0),
	}

	system := "Below is an instruction that describes a task. Write a response that appropriately completes the request.\n"

	output, err := llm.Predict(system+message, params...)
	if err != nil {
		fmt.Println(err)
	}
	lstart.Finish()

	return output
}*/
