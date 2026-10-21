// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build ruleguard

// Package gorules holds the gocritic ruleguard rules wired up in .golangci.yml.
// golangci-lint caches results without keying on this file, so run
// `bin/golangci-lint cache clean` after editing a rule.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// nonProductionPkg matches binaries and test-only harness packages, where a
// root context or a default HTTP client is acceptable.
const nonProductionPkg = `/v2/(build|cmd|evals|mcpserver/cmd|mcpserver/testhelpers|llm/llmtest)(/|$)`

func httpClientWithoutTimeout(m dsl.Matcher) {
	m.Match(`http.Client{$*fields}`, `&http.Client{$*fields}`).
		Where((m["$$"].Type.Is(`http.Client`) || m["$$"].Type.Is(`*http.Client`)) &&
			!m["fields"].Contains(`Timeout: $_`) &&
			!m.File().Name.Matches(`_test\.go$`) &&
			!m.File().PkgPath.Matches(nonProductionPkg)).
		Report(`http.Client without Timeout; set one, or suppress with a reason if a context deadline bounds every request`)

	m.Match(`http.DefaultClient`, `new(http.Client)`).
		Where(!m.File().Name.Matches(`_test\.go$`) && !m.File().PkgPath.Matches(nonProductionPkg)).
		Report(`$$ has no Timeout; use a client with one, or suppress with a reason if a context deadline bounds every request`)
}

func rootContextInProduction(m dsl.Matcher) {
	m.Match(`context.Background()`, `context.TODO()`).
		Where(!m.File().Name.Matches(`_test\.go$`) && !m.File().PkgPath.Matches(nonProductionPkg)).
		Report(`$$ drops cancellation and trace correlation; pass the caller's ctx (context.WithoutCancel for background work), or suppress with a reason at a true root`)
}

// testing.T's FailNow family must run on the test goroutine; an httptest
// handler runs on the server's goroutine. testifylint's go-require covers
// require.* the same way.
func failNowInHTTPHandler(m dsl.Matcher) {
	m.Match(`func($_ http.ResponseWriter, $_ *http.Request) { $*body }`).
		Where(m["body"].Contains(`$t.Fatal($*_)`) ||
			m["body"].Contains(`$t.Fatalf($*_)`) ||
			m["body"].Contains(`$t.FailNow()`) ||
			m["body"].Contains(`$t.Skip($*_)`) ||
			m["body"].Contains(`$t.Skipf($*_)`) ||
			m["body"].Contains(`$t.SkipNow()`)).
		Report(`t.Fatal/FailNow/Skip in an HTTP handler runs off the test goroutine; use t.Error or assert and return`)
}
