# Content Moderation Design Proposal

This document outlines a proposal for implementing content moderation using two Mattermost plugins:
1. A dedicated **Content Moderation Plugin** using Azure AI Content Safety APIs to moderate all content in the system
2. Updates to the **AI Plugin** to support moderation of LLM-generated content

The primary goal is to protect users from potentially harmful, offensive, or inappropriate content in both user-generated posts and AI-generated responses.

## Implementation Strategy

This design adopts a **minimum viable product (MVP)** approach for the first implementation of content moderation:

- Focus on essential functionality with a simple binary allow/reject model
- Implement core moderation through Mattermost plugin hooks for all content
- Use a global configuration with consistent thresholds
- Provide basic audit logging for moderation decisions
- Defer more complex features to future iterations

Throughout this document, we identify potential enhancements that could be considered for future releases after gathering customer feedback on the initial implementation.

## API Selection

### Azure Content Moderator (Deprecated)

Azure Content Moderator is deprecated as of February 2024 and will be retired by February 2027. Microsoft recommends migrating to Azure AI Content Safety, which offers more advanced AI features and enhanced performance. Due to this deprecation, we should avoid using the older Content Moderator APIs for any new implementation.

### Azure AI Content Safety (Recommended)

Azure AI Content Safety is a comprehensive solution designed to detect harmful user-generated and AI-generated content in applications and services. It is suitable for many scenarios such as online marketplaces, gaming companies, social messaging platforms, enterprise media companies, and K-12 education solution providers.

The Azure AI Content Safety APIs provide comprehensive content moderation capabilities:
- Text analysis for harmful content (hate speech, sexual content, violence, self-harm)
- Image analysis for inappropriate visual content
- Multiple severity levels (0-Safe, 2-Low, 4-Medium, 6-High)
- Support for blocklists and custom categories

The proposed design below utilizes the AI Content Safety APIs.

## Proposed Architecture

### Content Moderation Plugin

The new Content Moderation Plugin will:

1. Leverage Mattermost Server Plugin Hooks, specifically `MessageWillBePosted` and `MessageWillBeUpdated` to intercept all messages.
2. Implement a generic `Moderator` interface that defines methods for text and image moderation, allowing for different implementation providers.
3. Provide an Azure implementation of the `Moderator` interface that integrates with Azure AI Content Safety APIs.
4. Filter content based on configurable thresholds for different content categories.

### AI Plugin Updates

The AI Plugin will be updated to:

1. Add a new `disableStreaming` configuration option
2. Modify the streaming behavior to support content moderation 
3. Coordinate with the Content Moderation Plugin via direct post filtering

### Service Availability Considerations:
- The Content Moderation Plugin would use a "fail-closed" approach for safety - if the moderation service encounters an error, content would be blocked by default.
- This ensures potentially problematic content doesn't slip through due to technical issues.
- However, this means if moderation APIs are unavailable, all posting interactions would be disabled by default.

**Initial Implementation Approach:**
- For the first release, we will use the fail-closed approach for simplicity and security.

**Potential Future Considerations (Not in Initial Implementation):**
- Options for fail-open in trusted environments
- Graceful degradation mechanisms instead of complete service shutdown

## Proposed Moderation Flow

### Content Moderation Plugin

The Content Moderation Plugin will provide comprehensive moderation through these primary hooks:

1. **Message Posting Moderation**:
   - Uses `MessageWillBePosted` hook to intercept and check posts before they are created.
   - Only moderates posts from specific users (e.g., the AI bot) based on configuration.
   - If flagged, the user receives a notification and the message is blocked.
   - Particularly important for AI-generated responses to ensure they meet content standards.

2. **Message Update Moderation**:
   - Uses `MessageWillBeUpdated` hook to intercept and check post edits.
   - Same user-targeting logic applies - only monitors configured users.
   - If flagged, the edit is blocked and the original post remains unchanged.

3. **Attachment Moderation**:
   - Also within the `MessageWillBePosted` hook, checks file attachments including images.
   - Only applies to attachments from configured users.
   - Uses Azure AI Content Safety's image analysis capabilities to detect inappropriate visuals.

### AI Plugin Adjustments

The AI Plugin will be updated to work with the new moderation system:

1. **Streaming Behavior**:
   - The AI plugin will add a new `disableStreaming` configuration option.
   - When streaming is disabled:
     - A "working" message will be displayed while the complete response is generated
     - The complete response will be posted as a single message, which is then caught by the moderation plugin's `MessageWillBePosted` hook
     - If it passes moderation, the message is displayed to users
     - If it fails moderation, the moderation plugin will block it and the user will see the moderation failure message
     - The rejected content is never shown to the user

### Review Mechanism Considerations:
- If content exceeds the configured threshold, it would be automatically rejected without any human review process.
- This binary approach (allow/reject) will be used for the initial implementation.
- All rejected content will be audit logged (without including the actual content).

**Potential Future Enhancements (Not in Initial Implementation):**
- Review queues for borderline content
- Ability for moderators to override automated decisions
- Graduated response levels (warnings, override options, etc.)
- Feedback loops for users to contest moderation decisions

> **Note:** For the first iteration, we will focus on implementing a simple, minimal viable product before considering more complex review mechanisms. Additional features will be prioritized based on customer feedback after the initial release.

### User Feedback Considerations:
- For the initial implementation, users will receive generic messages when their content is flagged, without specifics about why.
- This simple approach will be sufficient for the MVP release.

**Potential Future Enhancements (Not in Initial Implementation):**
- More detailed feedback about content rejection reasons
- Different feedback levels based on user roles/permissions
- Educational resources about content policies

## Proposed Configuration

### Content Moderation Plugin Settings

The Content Moderation Plugin would be configured through its own plugin settings:

```json
{
  "enabled": true,
  "type": "azure",
  "endpoint": "https://*.azure.com",
  "apiKey": "your-api-key",
  "moderationTargets": {
    "users": ["ai-bot-user-id"],
    "moderateAllUsers": false
  },
  "thresholds": {
    "hate": 4,
    "sexual": 4,
    "violence": 4,
    "selfHarm": 4
  },
}
```

### AI Plugin Settings

The AI Plugin configuration would be updated to include only the streaming behavior setting:

```json
{
  "disableStreaming": true
}
```

These designs allow for:
- Future expansion to other moderation providers by changing the `type` parameter
- Category-specific thresholds based on Azure Content Safety's severity levels:
  - 0: Safe (always allowed)
  - 2: Low severity (mild)
  - 4: Medium severity (moderate)
  - 6: High severity (severe)
- Configuration control over which severity levels trigger moderation actions
- Separation of concerns between content generation and content moderation

### Threshold Configuration Considerations:
- This proposal uses category-specific thresholds (hate, sexual, violence, selfHarm) based on Azure Content Safety's severity levels.
- The default configuration would block content at medium severity (4) or higher.
- For the initial implementation, thresholds will be global and not configurable per channel or team.

**Potential Future Enhancements (Not in Initial Implementation):**
- Channel/Team-specific threshold configurations
- More sophisticated threshold testing and tuning capabilities
- Different actions based on severity levels (warnings vs. blocks)
- Blocklist integration for custom forbidden terms
- Jailbreak detection using Azure's Text Jailbreak Detection API
- Expand user targeting to include groups and teams

### Logging and Audit Considerations:
- All moderation decisions will be logged by the Content Moderation Plugin, but the actual content will not be included in logs.
- Any configuration changes (especially threshold value changes) must be audit logged.
- Normal logging will be used for non-sensitive information.
- Audit logging is mandatory for all content rejections.

### Plugin Integration Considerations:
- The Content Moderation Plugin operates independently but affects AI Plugin content.
- The AI Plugin only needs to be aware of moderation to the extent it affects streaming behavior.
- The Mattermost Server handles message rejection notifications to users.
- No direct inter-plugin communication is required as the moderation is handled through the standard message flow.

**Potential Future Enhancements (Not in Initial Implementation):**
- Admin dashboards for monitoring moderation activity
- Extended metadata for moderation decision logs
- Configurable log retention policies
- Inter-plugin communication to enable more sophisticated moderation coordination
