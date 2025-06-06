// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

// SaveTitleAsync saves a title asynchronously
func (c *Conversations) SaveTitleAsync(threadID, title string) {
	go func() {
		if err := c.SaveTitle(threadID, title); err != nil {
			c.mmClient.LogError("failed to save title: " + err.Error())
		}
	}()
}

// SaveTitle saves a title for a thread
func (c *Conversations) SaveTitle(threadID, title string) error {
	if c.db == nil {
		return nil // Skip database operations when db is not available
	}
	_, err := c.db.ExecBuilder(c.db.Builder().Insert("LLM_PostMeta").
		Columns("RootPostID", "Title").
		Values(threadID, title).
		Suffix("ON CONFLICT (RootPostID) DO UPDATE SET Title = ?", title))
	return err
}

// This is a different AIThread struct than the one in conversations.go, used for database queries
type aiThreadData struct {
	ID         string
	Message    string
	ChannelID  string
	Title      string
	ReplyCount int
	UpdateAt   int64
}

func (c *Conversations) getAIThreads(dmChannelIDs []string) ([]AIThread, error) {
	var dbPosts []aiThreadData
	if err := c.db.DoQuery(&dbPosts, c.db.Builder().
		Select(
			"p.Id",
			"p.Message",
			"p.ChannelID",
			"COALESCE(t.Title, '') as Title",
			"(SELECT COUNT(*) FROM Posts WHERE Posts.RootId = p.Id AND DeleteAt = 0) AS ReplyCount",
			"p.UpdateAt",
		).
		From("Posts as p").
		Where(sq.Eq{"ChannelID": dmChannelIDs}).
		Where(sq.Eq{"RootId": ""}).
		Where(sq.Eq{"DeleteAt": 0}).
		LeftJoin("LLM_PostMeta as t ON t.RootPostID = p.Id").
		OrderBy("CreateAt DESC").
		Limit(60).
		Offset(0),
	); err != nil {
		return nil, fmt.Errorf("failed to get posts for bot DM: %w", err)
	}

	// Convert from internal type to public AIThread type
	result := make([]AIThread, len(dbPosts))
	for i, post := range dbPosts {
		result[i] = AIThread{
			ID:        post.ID,
			Title:     post.Title,
			ChannelID: post.ChannelID,
			BotID:     "", // We don't have this info in the query
			UpdatedAt: post.UpdateAt,
		}
	}

	return result, nil
}
