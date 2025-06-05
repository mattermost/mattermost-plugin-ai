# Mattermost Agents User Guide

This guide explains how to use the AI features available through the Mattermost Agents plugin. This plugin transforms Mattermost into an AI-enhanced collaboration platform to improve team productivity and communication.

With Mattermost Agents, you can summarize call and meeting recordings, turn long threads and unread channel messages into concise summaries, stay on top of your messages by identifying next steps and decisions, extract learnings and transform content into charts and documentation, dig further into any topic by asking for insights, and leverage voice dictation tools for hands-free communication.

## Access AI features

You can access AI features in Mattermost in the following ways:

### Web and desktop

Access AI features through the right pane in one of the following ways:

- Select the **Agents** icon in the apps sidebar.
- @mention an AI bot in any channel where you have access (such as `@copilot`).
- Use the **AI Actions** menu by hovering over the first message in any conversation thread
- Use the **Ask AI** option in channels with unread messages.

If your Mattermost workspace has multiple Agent bots, switch between them by selecting the bot name in the top right corner of the Agents pane.

### Mobile

Start or open a direct message with the Agent bot. If your system admin has configured multiple bots, switch between them by starting or opening each bot by name.

## Conversational AI features

### Chat with agents

You can have conversations with Agents in several ways:

**Agents pane**: Use the Agents right-hand pane for a streamlined experience. Begin with suggested prompts, or engage in a private thread with an Agent for a tailored experience. If you have follow-up questions or need further insights, simply ask. You can also attach files for AI analysis or reference.

**Direct messages**: Start a direct message with an Agent bot to have a private conversation. Chat privately with an Agent in direct message threads like you would any other Mattermost user.

**Channel mentions**: [@mention](https://docs.mattermost.com/collaborate/mention-people.html) Agent bots by their username, such as `@copilot`, in any thread to bring Agents capabilities to your conversation. The bot responds in a thread to keep channels organized, and other team members can view and contribute to the conversation. An Agent can help extract information quickly or transform discussions into charts, resources, documentation, and more, and can find action items and open questions in new messages.

### Select a bot

If multiple Agent bots are configured for your Mattermost workspace, select your preferred bot in the Agents pane or @mention specific bots by name in channels.

### Tool approval and security

When Agents use external tools or integrations, you may be prompted to approve tool usage for security. When a tool is called, you'll see a card showing the tool name and description, arguments being passed to the tool, and **Approve/Reject** options.

For security, tool calls are only available in direct messages and each tool call requires explicit approval before execution. You can review tool arguments before approving, and tool results are shown after successful execution.

Available tools in direct messages include:

- Server search (semantic search across your Mattermost instance)
- User lookup (find information about Mattermost users)
- GitHub integration (the ability to fetch GitHub issues and pull requests requires the [GitHub plugin](https://docs.mattermost.com/integrate/github.html))
- [Jira integration](https://docs.mattermost.com/integrate/jira.html) (retrieve Jira issues from public instances)
- MCP tools (external tools provided by configured MCP servers if enabled).

> [!NOTE]
> Tool availability depends on your user permissions and system configuration.

## Analyze threads and channels

### Summarize discussion threads

To summarize a discussion thread:

1. Hover over the first message in any [conversation thread](https://docs.mattermost.com/collaborate/organize-conversations.html).
2. Select the **AI Actions** icon.
3. Select **Summarize Thread**. 

The thread summary is generated in the Agents pane, and only you can view the summary.

This is particularly useful for catching up on long discussions, creating meeting notes, and sharing outcomes with team members. You can also extract action items or find open questions in the same menu.

### Summarize unread channels

To summarize unread Mattermost channels:

1. Scroll to the **New Messages** cutoff line in a channel with unread messages.
2. Select **Ask AI**.
3. Select **Summarize new messages**. 

The channel summary is generated in the Agents pane, and only you can view the summary.

## Search with AI

Enterprise customers can enhance Mattermost [search](https://docs.mattermost.com/collaborate/search-for-messages.html) with AI capabilities.

Open the Agents pane from the right sidebar and use natural language to search for content (such as "find discussions about the new product launch"). The AI will find semantically relevant results, even if they don't contain the exact keywords, and results respect your permissions so you'll only see content you have access to.

This feature accelerates decision-making and improves information flows by making it easier to find relevant content across threads, channels, and teams.

> [!NOTE]
> - Semantic AI search requires a Mattermost Enterprise license.
> - AI search is an experimental feature. 
> - Contact your system admin if this feature isn't available for your Mattermost instance.

## Analyze Images

For AI models with vision capabilities, attach an image file to your message when chatting with an Agent to ask questions about the image or request analysis. The Agent responds based on the visual content.

> [!NOTE]
> Image analysis is a Beta feature. Your system admin must enable vision capabilities for your bot, and the underlying AI model must support vision features.

## Record Calls to Summarize Meetings

Mattermost Enterprise customers can leverage Mattermost Calls to turn meeting recordings into actionable summaries with a single action. Ensure key points of your calls and meetings are captured and shared easily, and share meeting insights with your team and the broader organization.

To summarize a Mattermost call recording:

1. [Start a call](https://docs.mattermost.com/collaborate/make-calls.html#start-a-call) in Mattermost and [record the call](https://docs.mattermost.com/collaborate/make-calls.html#record-a-call) during the meeting. 
2. Once the call ends and the call recording and transcription is ready, select the **Create meeting summary** option located directly above the call recording. 

The meeting summary is generated and shared as a direct message with the person who requested the meeting summary.

> [!NOTE]
> Both call recordings and recorded meeting summarization requires a Mattermost Enterprise license. Contact your system admin if these features aren't available for your Mattermost instance.

## Tips and best practices

This guide provides practical tips and best practices for getting the most out of your interactions with Mattermost Agents.

### Voice dictation integration

Enable your operating system's voice dictation or speech recognition tools for hands-free communication with Agents.

#### Windows

1. Start a direct message chat with Agents, and ensure your cursor is in the Mattermost message text box
2. Ensure your microphone is connected and working
3. Activate Microsoft Voice Typing by pressing the Windows key + H to open the voice typing tool
4. Start talking - Windows transforms your voice into text within Mattermost

#### macOS

1. Navigate to System Settings > Keyboard > Dictation and enable dictation
   - Ensure the Microphone source is set correctly
   - Specify the shortcut key you want to use to turn dictation on and off
2. Start a direct message chat with Agents, and ensure your cursor is in the Mattermost message text box
3. Turn dictation on with the shortcut key you configured, and then start talking - macOS transforms your voice into text within Mattermost

#### Linux

You'll need an open-source speech recognition tool for Linux, such as Simon, SpeechControl, or Julius. Once you have a speech recognition tool installed and working, enable it, start a direct message with Agents, and start talking.

### Best practices for AI interaction

When working with AI technology like Agents, it's important to understand that the process is often iterative. Using an iterative approach ensures that you leverage Agents to complement your work, leading to higher quality results. Here are some tips for being more effective with Agents:

#### Avoid a one-and-done mindset

Don't assume that the first output from Agents will be perfect. Instead, review and refine the content to ensure it meets your standards and needs. You can make corrections like "In the second section, remove mention of widgets. Add voice memos instead," make edits like "Remove Section 3" or "Switch Section 3 with Section 5," reduce unnecessary words by saying "Remove unnecessary phrases to make this more concise," or compact statements with "Condense this into a single paragraph."

#### Use AI as a tool, not a replacement

Treat the outputs generated by Agents as initial drafts. Agents can help you enhance your writing and analysis, not replace your own skills and judgment. Think of Agents as your high-tech assistant that can provide suggestions and help you brainstorm.

#### Iterate for quality

Go through multiple rounds of revisions to catch errors, improve clarity, and refine the content to better align with your goals. By continually reviewing and tweaking the outputs, you'll end up with more polished and accurate content, and maximize the value of Agents by producing professional-grade results.

### Effective prompting techniques

#### Be specific and clear

The more specific your requests, the better Agents can assist you. Instead of asking "Help me with this document," try "Review this meeting summary and identify any action items that need follow-up."

#### Provide context

When asking Agents to analyze or work with content, provide relevant background information. This helps the AI understand the purpose and deliver more targeted assistance.

#### Use follow-up questions

Take advantage of Agents' context memory by asking follow-up questions. If you're working on a project, you can build on previous conversations to refine and improve your work.

#### Request different perspectives

Ask Agents to approach problems from different angles or consider various stakeholder viewpoints to get more comprehensive insights.

### Workflow integration tips

#### Start small

Begin with simple tasks like summarizing threads or asking basic questions, then gradually incorporate more complex workflows as you become comfortable with the tool.

#### Combine features

Use multiple Agents features together. For example, search for relevant past discussions, summarize the findings, and then ask Agents to help you synthesize the information into actionable insights.

#### Document your learnings

Keep track of particularly effective prompts or workflows that work well for your team's needs. This helps establish consistent practices across your organization.
