# Mattermost Agents User Guide

This guide explains how to use the AI features provided by the Mattermost Agents plugin. The plugin transforms Mattermost into an AI-enhanced collaboration platform to improve team productivity and communication.

With Agents, you can summarize call and meeting recordings, turn long threads and unread channel messages into concise summaries, stay on top of your messages by identifying next steps and decisions, extract learnings and transform content into charts and documentation, dig further into any topic by asking for insights, and leverage voice dictation tools for hands-free communication.

## Accessing AI Features

There are multiple ways to interact with AI features in Mattermost:

### Web and Desktop Applications

You can access AI features through the right sidebar by clicking the Agents icon in the apps sidebar, mention an AI agent in any channel where you have access (like `@agents`), use the AI Actions menu by hovering over the first message in any conversation thread, or use the "Ask AI" option in channels with unread messages.

If your Mattermost workspace has multiple Agents, switch between them by selecting the agent name in the top right corner of the Agents panel.

### Mobile Applications

Start or open a direct message with the Agent bot. If your administrator has configured multiple agents, switch between them by starting or opening each agent by name. 

## Conversational Features

### Chatting with Agents

You can have conversations with Agents in several ways:

**Agents Panel**: Use the Agents right-hand sidebar for a streamlined experience. Begin with suggested prompts, or engage in a private thread with an Agent for a tailored experience. If you have follow-up questions or need further insights, simply ask. You can also attach files for AI analysis or reference.

**Direct Messages**: Start a DM with an agent to have a private conversation. Chat privately with an Agent in direct message threads like you would any other Mattermost user.

**Channel Mentions**: Invoke the power of Agents by @mentioning Agents by their username, like `@agents`, in any thread to bring Agents capabilities to your conversation. The agent responds in a thread to keep channels organized, and other team members can view and contribute to the conversation. An Agent can help extract information quickly or transform discussions into charts, resources, documentation, and more, and can find action items and open questions in new messages.

### Bot Selection

If multiple agents are configured, you can select your preferred agent in the Agents panel or mention specific agents by name in channels.

### Tool Approval and Security

When Agents use external tools or integrations, you may be prompted to approve tool usage for security. When a tool is called, you'll see a card showing the tool name and description, arguments being passed to the tool, and Approve/Reject buttons.

For security, tool calls are only available in direct messages and each tool call requires explicit approval before execution. You can review tool arguments before approving, and tool results are shown after successful execution.

Available tools in direct messages include server search (semantic search across your Mattermost instance), user lookup (find information about Mattermost users), GitHub integration (fetch GitHub issues and pull requests - requires GitHub plugin), Jira integration (retrieve Jira issues from public instances), and MCP tools (external tools provided by configured MCP servers if enabled).

**Note**: Tool availability depends on your permissions and administrator configuration.

## Thread and Channel Analysis

### Thread Summarization

To summarize a discussion thread, hover over the first message in any conversation thread, select the AI Actions icon, and select "Summarize Thread". The thread summary is generated in the Agents pane, and only you can view the summary.

This is particularly useful for catching up on long discussions, creating meeting notes, and sharing outcomes with team members. You can also extract action items or find open questions in the same menu.

### Channel Summarization

To summarize unread Mattermost channels, scroll to the "New Messages" cutoff in a channel with unread messages, select "Ask AI", and then select "Summarize new messages". The channel summary is generated in the Agents pane, and only you can view the summary.

When your system admin has configured multiple agents, you can switch between them by selecting one from the drop-down menu.

## Semantic Search (Enterprise, Experimental)

The Agents plugin enhances Mattermost's search with AI capabilities. Open the Agents panel from the right sidebar and use natural language to search for content (like "find discussions about the new product launch"). The AI will find semantically relevant results, even if they don't contain the exact keywords, and results respect your permissions so you'll only see content you have access to.

This feature accelerates decision-making and improves information flows by making it easier to find relevant content across threads, channels, and teams.

**Note**: Semantic search requires an Enterprise license and is currently experimental. Contact your administrator if this feature is not available.

## Image Analysis (BETA)

For AI models with vision capabilities, attach an image to your message when chatting with an Agent and ask questions about the image or request analysis. The Agent will respond based on the visual content.

**Note**: Image analysis is in BETA. Your administrator must enable vision capabilities for your agent, and the underlying AI model must support vision features.

## Call Recording and Meeting Summarization

Leverage Mattermost Calls to turn meeting recordings into actionable summaries with a single click. This feature ensures key points are captured and shared easily, enabling effective sharing of meeting insights with your team and the broader organization.

To summarize a Mattermost call recording, start a call in Mattermost and record the call during the meeting. Once the call ends and the call recording and transcription is ready, select the "Create meeting summary" option located directly above the call recording. The meeting summary is generated and shared as a direct message with the person who requested the meeting summary.

## Additional Resources

- [Usage Tips and Best Practices](usage_tips.md): Practical guidance for getting the most out of Agents
- For more information about how Agents manage LLM context and ensure data privacy, see the Agents Context Management documentation
