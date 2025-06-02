package prompts

import "embed"

//go:embed *.tmpl
var PromptsFolder embed.FS

//go:generate go run generate_prompt_vars.go
