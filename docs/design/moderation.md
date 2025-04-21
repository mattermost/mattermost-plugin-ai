# Content Moderation Design Proposal

This document outlines a proposal for implementing content moderation in the Mattermost AI Plugin using Azure AI Content Safety APIs. Here we explore how we might filter inappropriate content in both user inputs to LLMs and LLM-generated responses to protect users from potentially harmful, offensive, or inappropriate content.

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

**Open Questions:**
- Should there be an option for fail-open in certain trusted environments?
- How can we implement graceful degradation instead of complete service shutdown?

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

3. **Image Moderation**:
   - Images attached to messages would be moderated before being sent to vision-capable LLMs.
   - Images that fail moderation would not be included in the context sent to the LLM.
   - This happens in `PostToAIPost()` in `post_processing.go`.

### Review Mechanism Considerations:
- If content exceeds the configured threshold, it would be automatically rejected without any human review process.
- This binary approach (allow/reject) may be too rigid for some use cases.

**Open Questions:**
- Should we implement a review queue for borderline content?
- Should moderators have the ability to override automated decisions?
- How would a review process be integrated with Mattermost's permission system?

**Potential Enhancements:**
- **Human-in-the-Loop Review**: We could implement a review queue where content that falls within a "gray zone" (e.g., between 0.6-0.8 on the threshold) would be sent for human review before final determination.
- **Graduated Response**: Rather than a binary allow/block decision, we could implement multiple response levels:
  - Allow (below threshold)
  - Warn user but allow (borderline)
  - Block but allow override with explanation (moderately above threshold)
  - Block completely (significantly above threshold)
- **Feedback Loop**: We could create a mechanism for users to contest moderation decisions, providing a learning loop for the system and reducing false positives over time.

### User Feedback Considerations:
- Users receive generic messages when their content is flagged, without specifics about why.

**Open Questions:**
- Should users receive more detailed feedback about why their content was rejected?
- Could we implement different levels of feedback based on user roles/permissions?
- How can we balance transparency with avoiding instructing users on circumventing filters?

**Potential Enhancement:**
- **User Education**: We could provide educational resources about content policies when users have content rejected, helping them understand the guidelines.

## Proposed Configuration

The moderation system would be configured through a generic moderation object in the plugin settings:

```json
{
  "moderation": {
    "enabled": true,
    "type": "azure",
    "endpoint": "https://*.azure.com",
    "apiKey": "your-api-key",
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

**Open Questions:**
- Should thresholds be configurable per channel or per team?
- How can administrators test and tune thresholds effectively?
- Should we implement different actions based on different severity levels (e.g., warn vs. block)?

**Potential Enhancements:**
- **Blocklist Integration**: We could leverage Azure Content Safety's blocklist feature to allow administrators to create custom lists of forbidden terms or patterns.
- **Channel/Team-Specific Settings**: We could enable different moderation policies for different contexts - stricter in public channels, more lenient in private teams with trusted users.
- **Jailbreak Detection (Prompt Shield)**: We could utilize Azure's specialized Text Jailbreak Detection API for identifying prompt injection attacks. This API specifically detects prompts designed to manipulate or "jailbreak" an LLM's safety guardrails. The API returns a simple boolean result indicating whether a jailbreak attempt was detected.

### Logging and Audit Considerations:
- All moderation decisions should be logged

**Open Questions:**
- What additional metadata should be logged for moderation decisions?
- Should we implement an admin dashboard for monitoring moderation activity?
- How long should moderation logs be retained?
