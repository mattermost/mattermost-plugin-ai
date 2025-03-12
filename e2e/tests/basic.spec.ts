import { test, expect } from '@playwright/test';

import RunContainer from 'helpers/plugincontainer';
import MattermostContainer from 'helpers/mmcontainer';
import { MattermostPage } from 'helpers/mm';
import { AIPlugin } from 'helpers/ai-plugin';
import { OpenAIMockContainer, RunOpenAIMocks, responseTest, responseTest2, responseTest2Text, responseTestText } from 'helpers/openai-mock';

// Test configuration
const username = 'regularuser';
const password = 'regularuser';

let mattermost: MattermostContainer;
let openAIMock: OpenAIMockContainer;

// Setup for all tests in the file
test.beforeAll(async () => {
  mattermost = await RunContainer();
  openAIMock = await RunOpenAIMocks(mattermost.network);
});

// Cleanup after all tests
test.afterAll(async () => {
  await openAIMock.stop();
  await mattermost.stop();
});

// Common test setup
async function setupTestPage(page) {
  const mmPage = new MattermostPage(page);
  const aiPlugin = new AIPlugin(page);
  const url = mattermost.url();

  await mmPage.login(url, username, password);

  return { mmPage, aiPlugin };
}

// Test suites
test.describe('Plugin Installation', () => {
  test('Plugin was installed correctly', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();
    await expect(aiPlugin.appBarIcon).toBeVisible();
  });
});

test.describe('RHS Bot Interactions', () => {
  test('can send message and receive response', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();

    await openAIMock.addCompletionMock(responseTest);
    await aiPlugin.sendMessage('Hello!');
    await aiPlugin.waitForBotResponse(responseTestText);
  });

  test('regenerate button creates new response', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();

    // First response
    await openAIMock.addCompletionMock(responseTest);
    await aiPlugin.sendMessage('Hello!');
    await aiPlugin.waitForBotResponse(responseTestText);

    // Second response with regenerate
    await openAIMock.addCompletionMock(responseTest2);
    await aiPlugin.regenerateResponse();
    await aiPlugin.waitForBotResponse(responseTest2Text);
  });

  test('can switch between bots', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();
    await openAIMock.addCompletionMock(responseTest, "second");

    // Switch to second bot
    await aiPlugin.switchBot('Second Bot');

    await aiPlugin.sendMessage('Hello!');
    await expect(page.getByRole('button', { name: 'second', exact: true })).toBeVisible();
    await aiPlugin.waitForBotResponse(responseTestText);
  });
});

test.describe('Prompt Templates', () => {
  test('prompt templates replace text in textarea', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();

    // Clicking prompt template adds message
    await aiPlugin.usePromptTemplate('brainstorm');
    await aiPlugin.expectTextInTextarea('Brainstorm ideas about ');

    // Clicking without editing replaces the text
    await aiPlugin.usePromptTemplate('todo');
    await aiPlugin.expectTextInTextarea('Write a todo list about ');
  });
});

test.describe('Bot Mentions', () => {
  test('bot responds to channel mentions but ignores code blocks', async ({ page }) => {
    const { mmPage } = await setupTestPage(page);
    await openAIMock.addCompletionMock(responseTest);

    // Code block mention - should be ignored
    await mmPage.sendChannelMessage('`@mock` TestBotMention1');
    await mmPage.expectNoReply();

    // Multi-line code block mention - should be ignored
    await mmPage.sendChannelMessage('```\n@mock\n``` TestBotMention2');
    await mmPage.expectNoReply();

    // Regular mention - should get response
    await mmPage.mentionBot('mock', 'TestBotMention3');
    await mmPage.waitForReply();
  });
});

// Error handling tests
test.describe('Error Handling', () => {
  test('handles API errors gracefully', async ({ page }) => {
    const { aiPlugin } = await setupTestPage(page);
    await aiPlugin.openRHS();

    await openAIMock.addErrorMock(500, "Internal Server Error");
    await aiPlugin.sendMessage('This should cause an error');

    // Check if error message is displayed
    await expect(page.getByText(/An error occurred/i)).toBeVisible();
  });
});
