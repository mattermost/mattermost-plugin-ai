// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"encoding/json"
	"os"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

// ThreadExport represents the format of exported thread data
type ThreadExport struct {
	Thread    map[string]*model.Post     `json:"thread"`
	Channel   *model.Channel             `json:"channel"`
	Team      *model.Team                `json:"team"`
	Users     map[string]*model.User     `json:"users"`
	FileInfos map[string]*model.FileInfo `json:"file_infos"`
	Files     map[string][]byte          `json:"files"`

	// Helper fields not in the JSON
	RootPost *model.Post     `json:"-"`
	PostList *model.PostList `json:"-"`
}

// LoadThreadFromJSON loads post data from a JSON file containing exported Mattermost thread data
// and returns it as ThreadData containing Posts, RootPost, and PostList for testing
func LoadThreadFromJSON(t *EvalT, path string) *ThreadExport {
	// Open the JSON file
	jsonFile, err := os.ReadFile(path)
	require.NoError(t, err, "Failed to read test data file: %s", path)

	// Parse the JSON data
	var threadExport ThreadExport
	err = json.Unmarshal(jsonFile, &threadExport)
	require.NoError(t, err, "Failed to unmarshal JSON data")
	require.NotEmpty(t, threadExport.Thread, "No posts loaded from file")

	// Convert thread map to slice of posts
	posts := make([]*model.Post, 0, len(threadExport.Thread))
	for _, post := range threadExport.Thread {
		posts = append(posts, post)
	}

	// Find the root post (the one with empty root_id)
	var rootPost *model.Post
	for _, post := range posts {
		if post.RootId == "" {
			rootPost = post
			break
		}
	}
	require.NotNil(t, rootPost, "Root post not found in thread data")

	// Create a model.PostList from the posts
	postList := &model.PostList{
		Order: make([]string, len(posts)),
		Posts: make(map[string]*model.Post),
	}

	// Add all posts to the postList
	for i, post := range posts {
		postList.Order[i] = post.Id
		postList.Posts[post.Id] = post
	}

	threadExport.RootPost = rootPost
	threadExport.PostList = postList
	return &threadExport
}
