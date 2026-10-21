// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build ruleguard

// Package gorules holds the gocritic ruleguard rules wired up in .golangci.yml.
// golangci-lint caches results without keying on this file, so run
// `bin/golangci-lint cache clean` after editing a rule.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

func httpClientWithoutTimeout(m dsl.Matcher) {
	m.Match(`http.Client{$*fields}`, `&http.Client{$*fields}`).
		Where((m["$$"].Type.Is(`http.Client`) || m["$$"].Type.Is(`*http.Client`)) &&
			!m["fields"].Contains(`Timeout: $_`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report(`http.Client without Timeout; set one, or suppress with a reason if a context deadline bounds every request`)
}

// testing.T's FailNow family must run on the test goroutine; an httptest
// handler runs on the server's goroutine. testifylint's go-require covers
// require.* the same way.
func failNowInHTTPHandler(m dsl.Matcher) {
	m.Match(`func($_ http.ResponseWriter, $_ *http.Request) { $*body }`).
		Where(m["body"].Contains(`$t.Fatal($*_)`) ||
			m["body"].Contains(`$t.Fatalf($*_)`) ||
			m["body"].Contains(`$t.FailNow()`)).
		Report(`t.Fatal/FailNow in an HTTP handler runs off the test goroutine; use t.Error or assert and return`)
}
