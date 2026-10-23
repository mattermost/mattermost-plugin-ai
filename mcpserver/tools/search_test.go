// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/search"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSemanticSearchService is a simple test double for SemanticSearchService
type mockSemanticSearchService struct {
	enabled bool
	results []search.RAGResult
	err     error
}

func (m *mockSemanticSearchService) Enabled() bool { return m.enabled }

func (m *mockSemanticSearchService) Search(_ context.Context, _ string, _ search.Options) ([]search.RAGResult, error) {
	return m.results, m.err
}

func TestGetSearchTools_SchemaReflectsCapabilities(t *testing.T) {
	testCases := []struct {
		name                 string
		searchService        SemanticSearchService
		expectSemanticParams bool
		descriptionContains  string
	}{
		{
			name:                 "nil search service should produce keyword-only schema",
			searchService:        nil,
			expectSemanticParams: false,
			descriptionContains:  "keyword search",
		},
		{
			name:                 "disabled search service should produce keyword-only schema",
			searchService:        &mockSemanticSearchService{enabled: false},
			expectSemanticParams: false,
			descriptionContains:  "keyword search",
		},
		{
			name:                 "enabled search service should produce combined schema",
			searchService:        &mockSemanticSearchService{enabled: true},
			expectSemanticParams: true,
			descriptionContains:  "semantic (AI-powered) and keyword search",
		},
	}

	// Guidance fragments that must appear in every search_posts description so the
	// model does not combine username and display name into a single literal query
	// (see MM-67962).
	mentionGuidanceFragments := []string{
		"username only",
		"at-mentions in posts use the username",
		"Do not combine username and display name",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &MattermostToolProvider{
				logger:        &testLogger{t: t},
				searchService: tc.searchService,
				accessMode:    AccessModeRemote,
			}

			tools := provider.getSearchTools()
			require.NotEmpty(t, tools, "should return at least one tool")

			var searchPostsTool *MCPTool
			for i := range tools {
				if tools[i].Name == "search_posts" {
					searchPostsTool = &tools[i]
					break
				}
			}
			require.NotNil(t, searchPostsTool, "search_posts tool should exist")

			// Verify description matches capability
			assert.Contains(t, searchPostsTool.Description, tc.descriptionContains,
				"description should indicate correct search type")

			for _, fragment := range mentionGuidanceFragments {
				assert.Contains(t, searchPostsTool.Description, fragment,
					"description should include mention-search guidance: %q", fragment)
			}

			// Verify schema properties match capability
			require.NotNil(t, searchPostsTool.Schema, "schema should not be nil")
			require.NotNil(t, searchPostsTool.Schema.Properties, "schema should have properties")

			_, hasSemanticLimit := searchPostsTool.Schema.Properties["semantic_limit"]
			_, hasSemanticOffset := searchPostsTool.Schema.Properties["semantic_offset"]

			if tc.expectSemanticParams {
				assert.True(t, hasSemanticLimit, "combined schema should have semantic_limit")
				assert.True(t, hasSemanticOffset, "combined schema should have semantic_offset")
			} else {
				assert.False(t, hasSemanticLimit, "keyword-only schema should not have semantic_limit")
				assert.False(t, hasSemanticOffset, "keyword-only schema should not have semantic_offset")
			}

			// Both schemas should have keyword params
			_, hasKeywordLimit := searchPostsTool.Schema.Properties["keyword_limit"]
			_, hasKeywordOffset := searchPostsTool.Schema.Properties["keyword_offset"]
			assert.True(t, hasKeywordLimit, "schema should have keyword_limit")
			assert.True(t, hasKeywordOffset, "schema should have keyword_offset")

			// Both schemas should have the required query param
			_, hasQuery := searchPostsTool.Schema.Properties["query"]
			assert.True(t, hasQuery, "schema should have query parameter")
		})
	}
}

func TestFormatCombinedResults_Deduplication(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	duplicatePostID := "post123"

	semanticResults := []searchPostResult{
		{
			Post:        &model.Post{Id: duplicatePostID, ChannelId: "ch1", Message: "semantic message"},
			ChannelName: "General",
			Username:    "user1",
			Score:       0.95,
			Source:      "semantic",
		},
	}

	keywordResults := []searchPostResult{
		{
			Post:        &model.Post{Id: duplicatePostID, ChannelId: "ch1", Message: "keyword message"},
			ChannelName: "General",
			Username:    "user1",
			Source:      "keyword",
		},
		{
			Post:        &model.Post{Id: "uniquepost", ChannelId: "ch2", Message: "unique keyword message"},
			ChannelName: "Random",
			Username:    "user2",
			Source:      "keyword",
		},
	}

	result, err := provider.formatCombinedResults("test query", semanticResults, keywordResults, true, "")
	require.NoError(t, err)

	// Count occurrences of the duplicate post ID - should appear exactly once
	occurrences := strings.Count(result, duplicatePostID)
	assert.Equal(t, 1, occurrences,
		"duplicate post ID should appear exactly once after deduplication")

	// Verify the unique keyword result is still present
	assert.Contains(t, result, "uniquepost", "unique keyword result should be present")

	// Verify counts in the header are correct (1 semantic + 1 keyword after dedup)
	assert.Contains(t, result, "2 results", "should report 2 total results")
	assert.Contains(t, result, "1 semantic", "should report 1 semantic result")
	assert.Contains(t, result, "1 keyword", "should report 1 keyword result after dedup")
}

func TestFormatCombinedResults_DeduplicationPrefersSemantic(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	duplicatePostID := "post123"

	// Semantic result has more detail
	semanticResults := []searchPostResult{
		{
			Post:        &model.Post{Id: duplicatePostID, ChannelId: "ch1", Message: "detailed message"},
			ChannelName: "General",
			TeamName:    "MyTeam",
			Username:    "user1",
			Score:       0.95,
			Source:      "semantic",
		},
	}

	// Keyword result has less detail
	keywordResults := []searchPostResult{
		{
			Post:        &model.Post{Id: duplicatePostID, ChannelId: "ch1", Message: "brief"},
			ChannelName: "",
			Username:    "",
			Source:      "keyword",
		},
	}

	result, err := provider.formatCombinedResults("test query", semanticResults, keywordResults, true, "")
	require.NoError(t, err)

	// Verify the semantic result's details are in the output (not keyword's sparse data)
	assert.Contains(t, result, "detailed message", "should contain semantic result's message")
	assert.Contains(t, result, "Score: 0.95", "should contain semantic result's score")
	assert.Contains(t, result, "General", "should contain semantic result's channel name")
}

func TestFormatCombinedResults_ZeroResults(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	testCases := []struct {
		name            string
		semanticEnabled bool
	}{
		{
			name:            "zero results with semantic enabled",
			semanticEnabled: true,
		},
		{
			name:            "zero results with semantic disabled",
			semanticEnabled: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.formatCombinedResults("test query", nil, nil, tc.semanticEnabled, "")
			require.NoError(t, err)
			assert.NotEmpty(t, result, "should return a user-friendly message, not empty string")
			assert.Contains(t, result, "No posts found", "should indicate no results were found")
		})
	}
}

func TestFormatCombinedResults_EdgeCases(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	testCases := []struct {
		name            string
		semanticResults []searchPostResult
		keywordResults  []searchPostResult
		channelFilter   string
		checkFn         func(t *testing.T, result string)
	}{
		{
			name: "empty message field",
			keywordResults: []searchPostResult{
				{
					Post:     &model.Post{Id: "post1", ChannelId: "ch1", Message: ""},
					Username: "user1",
					Source:   "keyword",
				},
			},
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "post1", "should contain post ID even with empty message")
			},
		},
		{
			name: "unicode and emoji in content",
			keywordResults: []searchPostResult{
				{
					Post:        &model.Post{Id: "post1", ChannelId: "ch1", Message: "Hello 世界 🚀 émojis"},
					ChannelName: "日本語チャンネル",
					Username:    "用户",
					Source:      "keyword",
				},
			},
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "Hello 世界 🚀 émojis", "should preserve unicode in message")
				assert.Contains(t, result, "日本語チャンネル", "should preserve unicode in channel name")
				assert.Contains(t, result, "用户", "should preserve unicode in username")
			},
		},
		{
			name: "missing username shows Unknown User",
			keywordResults: []searchPostResult{
				{
					Post:     &model.Post{Id: "post1", ChannelId: "ch1", Message: "test"},
					Username: "",
					Source:   "keyword",
				},
			},
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "Unknown User", "should show Unknown User for empty username")
			},
		},
		{
			name: "channel filter is displayed",
			keywordResults: []searchPostResult{
				{
					Post:     &model.Post{Id: "post1", ChannelId: "channel123456789012345678", Message: "test"},
					Username: "user1",
					Source:   "keyword",
				},
			},
			channelFilter: "channel123456789012345678",
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "Channel ID filter:", "should indicate channel filter was applied")
				assert.Contains(t, result, "channel123456789012345678", "should show the filter value")
			},
		},
		{
			name: "thread reply shows root ID",
			keywordResults: []searchPostResult{
				{
					Post:     &model.Post{Id: "post1", ChannelId: "ch1", Message: "reply", RootId: "rootpost123"},
					Username: "user1",
					Source:   "keyword",
				},
			},
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "Root ID: rootpost123", "should show root ID for thread replies")
			},
		},
		{
			name: "semantic results show scores",
			semanticResults: []searchPostResult{
				{
					Post:     &model.Post{Id: "post1", ChannelId: "ch1", Message: "test"},
					Username: "user1",
					Score:    0.87,
					Source:   "semantic",
				},
			},
			checkFn: func(t *testing.T, result string) {
				assert.Contains(t, result, "Score: 0.87", "should display semantic score")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.formatCombinedResults("test query", tc.semanticResults, tc.keywordResults, true, tc.channelFilter)
			require.NoError(t, err)
			tc.checkFn(t, result)
		})
	}
}

func TestFormatCombinedResults_OnlySemanticResults(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	semanticResults := []searchPostResult{
		{
			Post:        &model.Post{Id: "post1", ChannelId: "ch1", Message: "semantic result"},
			ChannelName: "General",
			Username:    "user1",
			Score:       0.9,
			Source:      "semantic",
		},
	}

	result, err := provider.formatCombinedResults("test query", semanticResults, nil, true, "")
	require.NoError(t, err)

	assert.Contains(t, result, "1 result for", "should report 1 total result (singular)")
	assert.Contains(t, result, "1 semantic", "should report 1 semantic result")
	assert.Contains(t, result, "0 keyword", "should report 0 keyword results")
	assert.Contains(t, result, "Semantic Search Results", "should have semantic section")
}

func TestFormatCombinedResults_OnlyKeywordResults(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	keywordResults := []searchPostResult{
		{
			Post:        &model.Post{Id: "post1", ChannelId: "ch1", Message: "keyword result"},
			ChannelName: "General",
			Username:    "user1",
			Source:      "keyword",
		},
	}

	result, err := provider.formatCombinedResults("test query", nil, keywordResults, true, "")
	require.NoError(t, err)

	assert.Contains(t, result, "1 result for", "should report 1 total result (singular)")
	assert.Contains(t, result, "0 semantic", "should report 0 semantic results")
	assert.Contains(t, result, "1 keyword", "should report 1 keyword result")
	assert.Contains(t, result, "Keyword Search Results", "should have keyword section")
}

func TestFormatCombinedResults_KeywordOnlyMode(t *testing.T) {
	provider := &MattermostToolProvider{
		logger: &testLogger{t: t},
	}

	keywordResults := []searchPostResult{
		{
			Post:        &model.Post{Id: "post1", ChannelId: "ch1", Message: "keyword result"},
			ChannelName: "General",
			Username:    "user1",
			Source:      "keyword",
		},
	}

	// Call with semanticEnabled=false to simulate keyword-only mode
	result, err := provider.formatCombinedResults("test query", nil, keywordResults, false, "")
	require.NoError(t, err)

	// In keyword-only mode, the output format is simpler
	assert.NotContains(t, result, "Semantic Search Results",
		"keyword-only mode should not show semantic section header")
	assert.NotContains(t, result, "Keyword Search Results",
		"keyword-only mode should not label keyword section")
	assert.NotContains(t, result, "semantic",
		"keyword-only mode should not mention semantic search at all")

	// Should still contain the results
	assert.Contains(t, result, "post1", "should contain the post ID")
	assert.Contains(t, result, "keyword result", "should contain the message")
}

func TestBuildSearchTermWithChannel(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		channelName string
		expected    string
	}{
		{
			name:        "simple query with channel name",
			query:       "bug fix",
			channelName: "town-square",
			expected:    "in:town-square bug fix",
		},
		{
			name:        "channel name with hyphens",
			query:       "release notes",
			channelName: "release-announcements-2024",
			expected:    "in:release-announcements-2024 release notes",
		},
		{
			name:        "query already containing in: modifier",
			query:       "in:other-channel error",
			channelName: "dev",
			expected:    "in:dev in:other-channel error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchTermWithChannel(tt.query, tt.channelName)
			if got != tt.expected {
				t.Errorf("buildSearchTermWithChannel(%q, %q) = %q, want %q", tt.query, tt.channelName, got, tt.expected)
			}
		})
	}
}

func TestHasChannelFilter(t *testing.T) {
	tests := []struct {
		term string
		want bool
	}{
		{term: "migration", want: false},
		{term: "from:alice migration", want: false},
		{term: "in:town-square migration", want: true},
		{term: "-in:town-square migration", want: true},
		{term: "channel:town-square migration", want: true},
		{term: "IN:town-square", want: true},
		{term: "before:2024-01-31 after:2024-01-01 on:2024-01-15 migration", want: false},
		{term: "\"in:town-square\"", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			assert.Equal(t, tt.want, hasChannelFilter(tt.term))
		})
	}
}

func TestMergePostLists(t *testing.T) {
	postList := func(ids ...string) *model.PostList {
		list := model.NewPostList()
		for _, id := range ids {
			list.AddPost(&model.Post{Id: id})
			list.AddOrder(id)
		}
		return list
	}

	tests := []struct {
		name  string
		lists []*model.PostList
		want  []string
	}{
		{name: "no lists", want: nil},
		{name: "nil list is skipped", lists: []*model.PostList{nil, postList("a")}, want: []string{"a"}},
		{name: "disjoint lists", lists: []*model.PostList{postList("a", "b"), postList("c")}, want: []string{"a", "b", "c"}},
		{name: "post shared across lists appears once", lists: []*model.PostList{postList("a", "dm"), postList("dm", "b")}, want: []string{"a", "b", "dm"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, post := range mergePostLists(tt.lists...) {
				got = append(got, post.Id)
			}
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPagePostsNewestFirst(t *testing.T) {
	newPosts := func() []*model.Post {
		return []*model.Post{
			{Id: "old", CreateAt: 100},
			{Id: "newest", CreateAt: 300},
			{Id: "mid-a", CreateAt: 200},
			{Id: "mid-b", CreateAt: 200},
		}
	}

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []string
	}{
		{name: "all posts newest first with ID tiebreak", offset: 0, limit: 10, want: []string{"newest", "mid-b", "mid-a", "old"}},
		{name: "limit", offset: 0, limit: 2, want: []string{"newest", "mid-b"}},
		{name: "offset and limit", offset: 1, limit: 2, want: []string{"mid-b", "mid-a"}},
		{name: "offset past the end", offset: 4, limit: 10, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, post := range pagePostsNewestFirst(newPosts(), tt.offset, tt.limit) {
				got = append(got, post.Id)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSearchKeywordPosts verifies which search endpoints are used and how
// per-team results are merged, against a stub Mattermost API.
func TestSearchKeywordPosts(t *testing.T) {
	teamIDs := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = model.NewId()
		}
		return ids
	}
	twoTeams := teamIDs(2)
	manyTeams := teamIDs(maxKeywordSearchTeams + 3)

	tests := []struct {
		name         string
		teamID       string
		term         string
		userTeams    []string
		failTeam     bool
		wantCross    bool
		wantPostIDs  []string
		wantNote     bool
		wantErr      bool
		wantTeamHits int
	}{
		{
			name:         "team given uses only that team",
			teamID:       twoTeams[0],
			term:         "in:town-square migration",
			userTeams:    twoTeams,
			wantTeamHits: 1,
			wantPostIDs:  []string{"dm", "post-" + twoTeams[0]},
		},
		{
			name:        "no team and no channel filter uses cross-team search",
			term:        "from:alice after:2024-01-01 migration",
			userTeams:   twoTeams,
			wantCross:   true,
			wantPostIDs: []string{"cross"},
		},
		{
			name:         "no team with in searches every team and dedupes",
			term:         "in:town-square migration",
			userTeams:    twoTeams,
			wantTeamHits: 2,
			wantPostIDs:  []string{"dm", "post-" + twoTeams[0], "post-" + twoTeams[1]},
		},
		{
			name:        "no team and no team memberships falls back to cross-team search",
			term:        "in:town-square migration",
			userTeams:   nil,
			wantCross:   true,
			wantPostIDs: []string{"cross"},
		},
		{
			name:         "too many teams searches the first teams and adds a note",
			term:         "in:town-square migration",
			userTeams:    manyTeams,
			wantTeamHits: maxKeywordSearchTeams,
			wantNote:     true,
		},
		{
			name:      "failed team search is an error",
			term:      "in:town-square migration",
			userTeams: twoTeams,
			failTeam:  true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var crossHits int
			teamHits := map[string]int{}

			writePosts := func(w http.ResponseWriter, ids ...string) {
				list := model.NewPostList()
				for i, id := range ids {
					list.AddPost(&model.Post{Id: id, CreateAt: int64(i)})
					list.AddOrder(id)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(list)
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/v4/users/me/teams", func(w http.ResponseWriter, _ *http.Request) {
				teams := make([]*model.Team, 0, len(tt.userTeams))
				for _, id := range tt.userTeams {
					teams = append(teams, &model.Team{Id: id})
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(teams)
			})
			mux.HandleFunc("POST /api/v4/posts/search", func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				crossHits++
				mu.Unlock()
				writePosts(w, "cross")
			})
			mux.HandleFunc("POST /api/v4/teams/{team_id}/posts/search", func(w http.ResponseWriter, r *http.Request) {
				teamID := r.PathValue("team_id")
				mu.Lock()
				teamHits[teamID]++
				mu.Unlock()
				if tt.failTeam && teamID == tt.userTeams[1] {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				// Every team-scoped search also matches DMs, so "dm" repeats across teams.
				writePosts(w, "post-"+teamID, "dm")
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()

			provider := newTestProvider(t, ts.URL)
			posts, note, err := provider.searchKeywordPosts(t.Context(), newTestClient(ts.URL), tt.teamID, tt.term)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantCross, crossHits > 0, "cross-team endpoint usage")
			assert.Len(t, teamHits, tt.wantTeamHits, "number of teams searched")
			for teamID, hits := range teamHits {
				assert.Equalf(t, 1, hits, "team %s should be searched once", teamID)
			}
			assert.Equal(t, tt.wantNote, note != "", "note: %q", note)

			if tt.wantPostIDs != nil {
				var got []string
				for _, post := range posts {
					got = append(got, post.Id)
				}
				sort.Strings(got)
				want := append([]string(nil), tt.wantPostIDs...)
				sort.Strings(want)
				assert.Equal(t, want, got)
			}
		})
	}
}
