import { test, expect, type Page, type Locator } from '@playwright/test';
import MattermostContainer from 'helpers/mmcontainer';
import { expandToolActivity } from 'helpers/llmbot-post';
import { MattermostPage } from 'helpers/mm';
import {
    OpenAIMockContainer,
    RunOpenAIMocks,
    buildChatCompletionMockRule,
    buildToolCallResponse,
    buildTextResponse,
    titleGenerationMockRule,
} from 'helpers/openai-mock';
import RunSystemConsoleContainer, { adminUsername, adminPassword } from 'helpers/system-console-container';
import { MOCK_OAUTH_SERVER_NAME, MOCK_OAUTH_SERVER_URL, buildMCPOAuthMocks } from 'helpers/mcp-oauth-mock';

/**
 * Test Suite: Agents ask users to connect MCP servers they have not connected
 *
 * The mock MCP server always answers 401, so the requesting user is never
 * connected. The agent must still learn the server exists (the tool-call mock
 * only matches requests whose system prompt lists it) and can ask for a
 * connection, which renders a Connect card instead of a refusal.
 */

let mattermost: MattermostContainer;
let openAIMock: OpenAIMockContainer;

const reason = 'I need it to look up your open tickets.';
const requestArgs = JSON.stringify({ server: MOCK_OAUTH_SERVER_NAME, reason });

// Only present in the system prompt when the server is offered for connection.
const unconnectedPromptMatch = 'the user has not connected their own account to them yet';

async function openBotReply(page: Page, userMessage: string): Promise<Locator> {
    const mmPage = new MattermostPage(page);
    await mmPage.login(mattermost.url(), adminUsername, adminPassword);
    await mmPage.createAndNavigateToDMWithBot(mattermost, adminUsername, adminPassword, 'toolbot');
    await mmPage.mentionBot('toolbot', userMessage);

    const post = page.locator('.post').filter({
        has: page.locator('.post-message__text').getByText(`@toolbot ${userMessage}`, { exact: true }),
    }).last();
    await expect(post.getByText(/\d+ repl/i)).toBeVisible({ timeout: 30000 });
    await post.getByText(/\d+ repl/i).click();

    return page.locator('#rhsContainer').locator('[data-testid="llm-bot-post"]').last();
}

test.describe('MCP connection requests (Mocked LLM)', () => {
    test.beforeAll(async () => {
        test.setTimeout(180000);
        mattermost = await RunSystemConsoleContainer({
            // mentionBot @-mentions the agent, which routes through the mention path.
            enableChannelMentionToolCalling: true,
            services: [
                {
                    id: 'mock-service',
                    name: 'Mock Service',
                    type: 'openaicompatible',
                    apiKey: 'mock',
                    apiURL: 'http://openai:8080',
                    defaultModel: 'gpt-mock',
                    useResponsesAPI: false,
                },
            ],
            bots: [
                {
                    id: 'tool-test-bot',
                    name: 'toolbot',
                    displayName: 'Tool Test Bot',
                    serviceID: 'mock-service',
                    customInstructions: '',
                    disableTools: false,
                    mcpDynamicToolLoading: false,
                    enabledNativeTools: [],
                },
            ],
            mcp: {
                enabled: true,
                enablePluginServer: true,
                embeddedServer: { enabled: true },
                idleTimeoutMinutes: 30,
                servers: [
                    {
                        name: MOCK_OAUTH_SERVER_NAME,
                        enabled: true,
                        baseURL: MOCK_OAUTH_SERVER_URL,
                        clientID: 'test-client-id',
                        clientSecret: 'test-client-secret',
                    },
                ],
            },
        });
        openAIMock = await RunOpenAIMocks(mattermost.network);
    });

    test.afterAll(async () => {
        await openAIMock.stop();
        await mattermost.stop();
    });

    test('skipping the request lets the agent continue without the server', async ({ page }) => {
        test.setTimeout(120000);

        const followUpText = 'Understood, continuing without the ticket tracker.';
        // addMocks resets Smocker, so the OAuth server mocks are registered with every turn.
        await openAIMock.addMocks([
            ...buildMCPOAuthMocks(),
            buildChatCompletionMockRule(
                buildToolCallResponse('call_connect_skip', 'RequestMCPConnection', requestArgs),
                { bodyContains: unconnectedPromptMatch },
            ),
            titleGenerationMockRule(),
            buildChatCompletionMockRule(buildTextResponse(followUpText), {
                bodyContains: `chose not to connect ${MOCK_OAUTH_SERVER_NAME}`,
            }),
        ]);

        const botPost = await openBotReply(page, 'Summarize my open tickets ' + Date.now());
        const card = botPost.getByTestId('connection-request-card');

        await expect(card.getByText(`Connect ${MOCK_OAUTH_SERVER_NAME}`, { exact: true })).toBeVisible({ timeout: 30000 });
        await expect(card.getByText(reason, { exact: true })).toBeVisible();
        await expect(card.getByRole('button', { name: /^connect$/i })).toBeVisible({ timeout: 30000 });
        await expect(card.getByRole('button', { name: /^continue$/i })).toHaveCount(0);

        await card.getByRole('button', { name: /^skip$/i }).click();

        await expect(page.locator('#rhsContainer').getByText(followUpText, { exact: false })).toBeVisible({ timeout: 30000 });
        const activityRounds = await expandToolActivity(botPost);
        await expect(activityRounds.getByText('Skipped', { exact: true })).toBeVisible({ timeout: 30000 });
    });

    test('Connect opens the OAuth start route and Continue reports the result back to the agent', async ({ page, context }) => {
        test.setTimeout(120000);

        const followUpText = 'The ticket tracker is still not connected, so here is what I can do instead.';
        await openAIMock.addMocks([
            ...buildMCPOAuthMocks(),
            buildChatCompletionMockRule(
                buildToolCallResponse('call_connect_continue', 'RequestMCPConnection', requestArgs),
                { bodyContains: unconnectedPromptMatch },
            ),
            titleGenerationMockRule(),
            buildChatCompletionMockRule(buildTextResponse(followUpText), {
                bodyContains: `${MOCK_OAUTH_SERVER_NAME} is still not connected`,
            }),
        ]);

        const botPost = await openBotReply(page, 'Find tickets assigned to me ' + Date.now());
        const card = botPost.getByTestId('connection-request-card');
        const connect = card.getByRole('button', { name: /^connect$/i });
        await expect(connect).toBeVisible({ timeout: 30000 });

        const startRequest = context.waitForEvent('request', (request) => request.url().includes('/mcp/oauth/'));
        await connect.click();
        const startURL = new URL((await startRequest).url());
        expect(startURL.origin).toBe(mattermost.url());
        expect(startURL.pathname).toBe(`/plugins/mattermost-ai/mcp/oauth/${encodeURIComponent(MOCK_OAUTH_SERVER_NAME)}/start`);

        // The OAuth flow cannot complete against the mock, so Continue resolves
        // to "not connected" after the server re-checks the catalog.
        await card.getByRole('button', { name: /^continue$/i }).click();

        await expect(page.locator('#rhsContainer').getByText(followUpText, { exact: false })).toBeVisible({ timeout: 30000 });
        const activityRounds = await expandToolActivity(botPost);
        await expect(activityRounds.getByText('Not connected', { exact: true })).toBeVisible({ timeout: 30000 });
    });
});
