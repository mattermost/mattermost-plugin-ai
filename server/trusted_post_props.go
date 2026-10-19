// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"reflect"

	"github.com/mattermost/mattermost-plugin-agents/v2/conversations"
	"github.com/mattermost/mattermost-plugin-agents/v2/meetings"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// pluginTrustedPostProps are props the plugin writes only onto its own agent
// bots' posts and that the plugin or webapp treat as trusted (conversation
// linkage, interruption notice, analysis/meeting references). A user must not
// be able to forge them on their own posts. Generic or safety-reducing props
// (e.g. unsafe_links, no_regen) are intentionally not listed.
var pluginTrustedPostProps = []string{
	streaming.InterruptedProp,
	streaming.InterruptedAtProp,
	streaming.ConversationIDProp,
	streaming.RespondingToProp,
	conversations.ThreadIDProp,
	meetings.ReferencedTranscriptPostID,
	meetings.ReferencedRecordingFileID,
}

// stripTrustedProps removes plugin-trusted props from a post unless it is
// authored by one of this plugin's agent bots. On an update, a prop whose
// value is unchanged from oldPost is kept: it was accepted when first written,
// and the local bot cache may not (yet, or any more) know the post's author.
// Returns the post to persist.
func stripTrustedProps(post, oldPost *model.Post, isAgentBot func(userID string) bool) *model.Post {
	if post == nil || isAgentBot(post.UserId) {
		return post
	}
	var stripped *model.Post
	for _, key := range pluginTrustedPostProps {
		value := post.GetProp(key)
		if value == nil {
			continue
		}
		if oldPost != nil && reflect.DeepEqual(oldPost.GetProp(key), value) {
			continue
		}
		if stripped == nil {
			stripped = post.Clone()
		}
		stripped.DelProp(key)
	}
	if stripped == nil {
		return post
	}
	return stripped
}

func (p *Plugin) isAgentBot(userID string) bool {
	return p.bots != nil && p.bots.IsAnyBot(userID)
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return stripTrustedProps(post, nil, p.isAgentBot), ""
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost, oldPost *model.Post) (*model.Post, string) {
	return stripTrustedProps(newPost, oldPost, p.isAgentBot), ""
}
