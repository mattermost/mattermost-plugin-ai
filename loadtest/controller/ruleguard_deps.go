// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build ruleguard

// The shared .golangci.yml loads build/ruleguard/rules.go, whose dsl import
// resolves through the module being linted.
package controller

import _ "github.com/quasilyte/go-ruleguard/dsl"
