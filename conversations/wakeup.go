// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-agents/v2/bots"
	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmtools"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost-plugin-agents/v2/telemetry"
	"github.com/mattermost/mattermost/server/public/model"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WakeJobKeyPrefix namespaces wait_for_async_work jobs in the shared
// cluster.JobOnceScheduler.
const WakeJobKeyPrefix = "wait_wake_"

// maxConsecutiveWakes bounds a wait chain the requester has not re-engaged
// with; the model-facing stop instruction in WakeUserMessage is advisory only.
const maxConsecutiveWakes = 20

// pendingWakeKeyPrefix records, per conversation, the job key of the only
// wake allowed to fire, so repeated or batched waits cannot fan out parallel
// resumes. A marker rather than a deterministic job key: JobOnce cannot
// safely cancel and reschedule one key across cluster nodes.
const pendingWakeKeyPrefix = "wait_pending_wake_"

// pendingWakeTTL outlives the longest wait plus scheduler downtime.
const pendingWakeTTL = 24 * time.Hour

// resumeLeaseKeyPrefix serializes wake resumes per conversation across the
// cluster, so a wake scheduled by a still-running resume waits for it.
const resumeLeaseKeyPrefix = "wait_resume_lease_"

// resumeLeaseTTL is renewed while a resume runs; it only bounds how long a
// crashed node can keep holding the lease.
const resumeLeaseTTL = 2 * time.Minute

// resumeLeaseMaxWait bounds how long a wake waits behind a running resume.
const resumeLeaseMaxWait = time.Hour

var (
	resumeLeasePollInterval  = 5 * time.Second
	resumeLeaseRenewInterval = resumeLeaseTTL / 3
)

// acquireResumeLease blocks until it holds the conversation's resume lease and
// keeps renewing it until the returned release func is called.
func (c *Conversations) acquireResumeLease(conversationID string) (func(), error) {
	key := resumeLeaseKeyPrefix + conversationID
	leaseID := model.NewId()
	deadline := time.Now().Add(resumeLeaseMaxWait)
	for {
		acquired, err := c.mmClient.KVCompareAndSetWithExpiry(key, nil, leaseID, resumeLeaseTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire resume lease: %w", err)
		}
		if acquired {
			stop := make(chan struct{})
			renewDone := make(chan struct{})
			go func() {
				defer close(renewDone)
				c.renewResumeLease(key, leaseID, resumeLeaseRenewInterval, stop)
			}()
			return func() {
				close(stop)
				<-renewDone
				if _, err := c.mmClient.KVCompareAndSet(key, leaseID, nil); err != nil {
					c.mmClient.LogWarn("wait_for_async_work: failed to release resume lease", "error", err, "conversation_id", conversationID)
				}
			}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for resume lease")
		}
		time.Sleep(resumeLeasePollInterval)
	}
}

func (c *Conversations) renewResumeLease(key, leaseID string, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			renewed, err := c.mmClient.KVCompareAndSetWithExpiry(key, leaseID, leaseID, resumeLeaseTTL)
			if err != nil || !renewed {
				c.mmClient.LogWarn("wait_for_async_work: failed to renew resume lease", "error", err, "key", key)
			}
		}
	}
}

// WakeJob is the JobOnce props payload for a wait_for_async_work resume. All
// other conversation state is rederived from the response post at wake time.
type WakeJob struct {
	PostID string `json:"post_id"`
	// ConversationID is empty for jobs scheduled before pending-wake tracking.
	ConversationID string `json:"conversation_id,omitempty"`
	// Reason is model-authored; never interpolate it into a user-role turn.
	Reason string `json:"reason"`
}

// RecordPendingWake marks jobKey as the conversation's only live wake,
// superseding any wake scheduled earlier.
func (c *Conversations) RecordPendingWake(conversationID, jobKey string) error {
	return c.mmClient.KVSetWithExpiry(pendingWakeKeyPrefix+conversationID, jobKey, pendingWakeTTL)
}

func (c *Conversations) isPendingWake(job WakeJob, jobKey string) (bool, error) {
	if job.ConversationID == "" {
		return true, nil
	}
	var pending string
	if err := c.mmClient.KVGet(pendingWakeKeyPrefix+job.ConversationID, &pending); err != nil {
		if mmapi.IsKVNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return pending == jobKey, nil
}

// HandleWakeJob is the cluster.JobOnceScheduler callback. Props scheduled
// before a restart arrive as generic JSON, so normalize via re-marshal.
func (c *Conversations) HandleWakeJob(key string, props any) {
	if !strings.HasPrefix(key, WakeJobKeyPrefix) {
		return
	}

	job, err := wakeJobFromProps(props)
	if err != nil {
		c.mmClient.LogError("wait_for_async_work: invalid wake job props", "error", err, "key", key)
		return
	}

	if !c.isCurrentWake(job, key) {
		return
	}

	// The scheduler serializes callbacks under a process-wide mutex; resume
	// runs a full LLM turn (potentially minutes), so it must not block other
	// wakes from firing.
	go c.runWake(job, key)
}

// runWake resumes the conversation once it holds the conversation's resume
// lease, re-checking afterwards that no newer wait superseded this wake.
func (c *Conversations) runWake(job WakeJob, key string) {
	if job.ConversationID != "" {
		release, err := c.acquireResumeLease(job.ConversationID)
		if err != nil {
			c.mmClient.LogError("wait_for_async_work: dropping wake", "error", err, "conversation_id", job.ConversationID)
			return
		}
		defer release()
		if !c.isCurrentWake(job, key) {
			return
		}
	}
	if err := c.ResumeConversation(context.Background(), job); err != nil {
		c.mmClient.LogError("wait_for_async_work: wake failed", "error", err, "post_id", job.PostID)
	}
}

func (c *Conversations) isCurrentWake(job WakeJob, key string) bool {
	pending, err := c.isPendingWake(job, key)
	if err != nil {
		c.mmClient.LogError("wait_for_async_work: failed to read pending wake", "error", err, "key", key)
		return false
	}
	if !pending {
		c.mmClient.LogDebug("wait_for_async_work: dropping superseded wake", "key", key, "conversation_id", job.ConversationID)
	}
	return pending
}

func wakeJobFromProps(props any) (WakeJob, error) {
	raw, err := json.Marshal(props)
	if err != nil {
		return WakeJob{}, fmt.Errorf("failed to marshal wake job props: %w", err)
	}
	var job WakeJob
	if err := json.Unmarshal(raw, &job); err != nil {
		return WakeJob{}, fmt.Errorf("failed to unmarshal wake job props: %w", err)
	}
	if job.PostID == "" {
		return WakeJob{}, fmt.Errorf("wake job props missing post_id")
	}
	return job, nil
}

// ResumeConversation injects a synthetic user turn for a wait_for_async_work
// wake and re-enters the tool follow-up loop on a new bot post in the same
// thread. Streaming onto the original post would blank and replace its
// finished response (continuation semantics), so the wake gets its own reply.
// Conversation state is rederived from the original response post, mirroring
// HandleToolCall; if any entity is gone the thread is dead and the wake is
// dropped.
func (c *Conversations) ResumeConversation(ctx context.Context, job WakeJob) error {
	post, err := c.mmClient.GetPost(job.PostID)
	if err != nil {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, post missing",
			"error", err, "post_id", job.PostID)
		return nil
	}

	// Chain into the originating run's trace, same as HandleToolCall / regen.
	// The scheduler callback has no request ctx; Background is independent of
	// the expired tool-call request. rehydrateRunTrace is best-effort.
	ctx = c.rehydrateRunTrace(ctx, post)
	ctx, span := telemetry.Tracer().Start(ctx, "resume conversation after wait",
		trace.WithNewRoot(),
		trace.WithAttributes(telemetry.PostID.String(job.PostID)),
	)
	defer span.End()

	convID, ok := post.GetProp(streaming.ConversationIDProp).(string)
	if !ok || convID == "" {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, post has no conversation",
			"post_id", job.PostID)
		return nil
	}

	conv, err := c.convService.GetConversation(convID)
	if err != nil {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, conversation missing",
			"error", err, "conversation_id", convID)
		return nil
	}

	bot := c.bots.GetBotByID(post.UserId)
	if bot == nil {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, bot missing", "bot_id", post.UserId)
		return nil
	}

	user, err := c.mmClient.GetUser(conv.UserID)
	if err != nil {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, user missing",
			"error", err, "user_id", conv.UserID)
		return nil
	}

	channel, err := c.mmClient.GetChannel(post.ChannelId)
	if err != nil {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, channel missing",
			"error", err, "channel_id", post.ChannelId)
		return nil
	}

	// The wake runs unattended with plugin authority, so re-check that the
	// requester could still start this turn themselves.
	if user.DeleteAt != 0 || !c.mmClient.HasPermissionToChannel(user.Id, channel.Id, model.PermissionReadChannel) {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, user no longer authorized",
			"user_id", user.Id, "channel_id", channel.Id)
		return nil
	}

	isDM := mmapi.IsDMWith(bot.GetMMBot().UserId, channel)
	var restrictionErr error
	if isDM {
		restrictionErr = c.bots.CheckUsageRestrictionsForUser(ctx, bot, user.Id)
	} else {
		restrictionErr = c.bots.CheckUsageRestrictions(ctx, user.Id, bot, channel)
	}
	if errors.Is(restrictionErr, bots.ErrUsageRestriction) {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, usage restricted",
			"error", restrictionErr, "user_id", user.Id, "bot_id", bot.GetMMBot().UserId, "channel_id", channel.Id)
		return nil
	} else if restrictionErr != nil {
		span.RecordError(restrictionErr)
		span.SetStatus(codes.Error, "failed to check usage restrictions")
		return fmt.Errorf("failed to check usage restrictions: %w", restrictionErr)
	}

	wakes, err := c.consecutiveWakeCount(conv.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to count wakes")
		return err
	}
	if wakes >= maxConsecutiveWakes {
		c.mmClient.LogDebug("wait_for_async_work: dropping wake, chain limit reached",
			"conversation_id", conv.ID, "wakes", wakes)
		return nil
	}

	responseRootID := post.Id
	if post.RootId != "" {
		responseRootID = post.RootId
	}
	wakePost := &model.Post{
		ChannelId: post.ChannelId,
		RootId:    responseRootID,
	}
	if err := c.createResponsePlaceholder(bot.GetMMBot().UserId, conv.UserID, wakePost, post.Id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create wake response post")
		return err
	}
	wakePost.AddProp(streaming.ConversationIDProp, conv.ID)
	if err := c.mmClient.UpdatePost(wakePost); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to attach conversation to wake response post")
		c.failResponsePlaceholder(wakePost, user.Locale)
		return fmt.Errorf("failed to attach conversation to wake response post: %w", err)
	}

	if err := c.convService.AppendSyntheticUserTurn(conv.ID, mmtools.WakeUserMessage); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write wake turn")
		c.failResponsePlaceholder(wakePost, user.Locale)
		return err
	}

	if err := c.streamToolFollowUp(ctx, bot, user, channel, wakePost, conv, isDM, true, false, nil); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to resume conversation")
		c.failResponsePlaceholder(wakePost, user.Locale)
		return err
	}
	return nil
}

// consecutiveWakeCount counts wake turns since the requester's latest real
// (post-linked) user turn.
func (c *Conversations) consecutiveWakeCount(conversationID string) (int, error) {
	turns, err := c.convService.GetTurns(conversationID)
	if err != nil {
		return 0, fmt.Errorf("failed to get turns: %w", err)
	}
	count := 0
	for _, turn := range turns {
		if turn.Role != "user" {
			continue
		}
		if turn.PostID != nil {
			count = 0
			continue
		}
		var blocks []conversation.ContentBlock
		if json.Unmarshal(turn.Content, &blocks) == nil && len(blocks) == 1 && blocks[0].Text == mmtools.WakeUserMessage {
			count++
		}
	}
	return count, nil
}
