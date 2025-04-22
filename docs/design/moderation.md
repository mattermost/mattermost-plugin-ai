# Content Moderation Design Proposal

This document outlines a proposal for implementing content moderation in the Mattermost AI Plugin using Azure AI Content Safety APIs. Here we explore how we might filter inappropriate content in both user inputs to LLMs and LLM-generated responses to protect users from potentially harmful, offensive, or inappropriate content.

## Implementation Strategy

This design adopts a **minimum viable product (MVP)** approach for the first implementation of content moderation:

- Focus on essential functionality with a simple binary allow/reject model
- Implement core moderation points (user input, LLM responses, image content)
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

The proposed moderation system would consist of the following components:

1. **Moderation Interface**: A generic `Moderator` interface that defines methods for text and image moderation, allowing for different implementation providers.
2. **Azure Implementation**: An implementation of the `Moderator` interface that integrates with Azure AI Content Safety APIs.
3. **Integration Points**: Code that integrates moderation into the message processing flow, both for user input and LLM responses.

### Service Availability Considerations:
- The proposed design would use a "fail-closed" approach for safety - if the moderation service encounters an error, content would be blocked by default.
- This ensures potentially problematic content doesn't slip through due to technical issues.
- However, this means if moderation APIs are unavailable, all LLM interactions would be disabled by default.

**Initial Implementation Approach:**
- For the first release, we will use the fail-closed approach for simplicity and security.

**Potential Future Considerations (Not in Initial Implementation):**
- Options for fail-open in trusted environments
- Graceful degradation mechanisms instead of complete service shutdown

## Proposed Moderation Flow

The proposed implementation would include three primary moderation points:

1. **User Input Moderation**:
   - When a user sends a message to an AI bot, the message would be checked by the moderation service.
   - If flagged, the user would receive a notification and the message would not be processed by the LLM.
   - This happens in `processUserRequestToBot()` in `conversations.go`.

2. **LLM Response Moderation**:
   - When an LLM generates a response, the complete response would be checked before being displayed to users.
   - If flagged, users would see a generic message instead of the problematic content.
   - This happens in `streamResultToPost()` in `post_processing.go`.
   
   **Streaming Considerations:**
   - The AI plugin normally streams LLM responses incrementally as they're generated, updating a post in real-time.
   - This creates a challenge for content moderation as the entire response must be evaluated.
   - A new setting will be introduced to disable streaming when moderation is enabled:
     - When streaming is disabled, a "working" message will be displayed while the complete response is generated
     - The complete response will then be checked by the moderation service
     - If it passes moderation, the "working" message will be replaced with the full response
     - If it fails moderation, the "working" message will be replaced with a moderation failure message
     - The rejected content is never shown to the user

3. **Image Moderation**:
   - Images attached to messages would be moderated before being sent to vision-capable LLMs.
   - Images that fail moderation would not be included in the context sent to the LLM.
   - This happens in `PostToAIPost()` in `post_processing.go`.

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

The moderation system would be configured through a generic moderation object in the plugin settings:

```json
{
  "moderation": {
    "enabled": true,
    "type": "azure",
    "endpoint": "https://*.azure.com",
    "apiKey": "your-api-key",
    "disableStreaming": true,
    "thresholds": {
      "hate": 4,
      "sexual": 4,
      "violence": 4,
      "selfHarm": 4
    }
  }
}
```

This design allows for:
- Future expansion to other moderation providers by changing the `type` parameter
- Category-specific thresholds based on Azure Content Safety's severity levels:
  - 0: Safe (always allowed)
  - 2: Low severity (mild)
  - 4: Medium severity (moderate)
  - 6: High severity (severe)
- Configuration control over which severity levels trigger moderation actions

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

### Logging and Audit Considerations:
- All moderation decisions will be logged, but the actual content will not be included in logs.
- Any configuration changes (especially threshold value changes) must be audit logged.
- Normal logging will be used for non-sensitive information.
- Audit logging is mandatory for all content rejections.

**Potential Future Enhancements (Not in Initial Implementation):**
- Admin dashboards for monitoring moderation activity
- Extended metadata for moderation decision logs
- Configurable log retention policies
