// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package indexing

import "github.com/mattermost/mattermost/server/public/model"

// shouldIndexPost returns whether a post should be indexed based on consistent criteria
func (s *Service) shouldIndexPost(post *model.Post, channel *model.Channel) bool {
	// Skip posts that don't have content
	if post.Message == "" {
		return false
	}

	// Skip posts from bots
	if s.bots.IsAnyBot(post.UserId) {
		return false
	}

	// Skip non-regular posts
	if post.Type != model.PostTypeDefault {
		return false
	}

	// Skip deleted posts
	if post.DeleteAt != 0 {
		return false
	}

	// Skip posts in DM channels with the bots
	if channel != nil && s.bots.GetBotForDMChannel(channel) != nil {
		return false
	}

	return true
}
