// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package prompts

import "embed"

//go:embed *.tmpl
var PromptsFolder embed.FS

//go:generate go run generate_prompt_vars.go
