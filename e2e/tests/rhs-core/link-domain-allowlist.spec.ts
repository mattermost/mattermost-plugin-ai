import path from 'path';

import { test, expect, Locator, Page } from '@playwright/test';

import MattermostContainer from 'helpers/mmcontainer';
import { MattermostPage } from 'helpers/mm';
import { AIPlugin } from 'helpers/ai-plugin';
import { SystemConsoleHelper } from 'helpers/system-console';
import { RunAIMockContainer } from 'helpers/plugincontainer';
import { mattermostAIAdminConfigApiFromClient } from 'helpers/plugin-http';
import { OpenAIMockContainer, RunOpenAIMocks, turnMocksWithTitleSiphon } from 'helpers/openai-mock';

const username = 'regularuser';
const password = 'regularuser';
const adminUsername = 'admin';
const adminPassword = 'admin';

const ALLOWED_LINK_DOMAINS = 'allowed.example.com, *.trusted.example';
const REPLY_MARKER = 'Link allowlist check';
const REPLY_MARKDOWN = [
    REPLY_MARKER,
    '',
    '- [Allowed](https://allowed.example.com/page)',
    '- [Sub](https://docs.trusted.example/x)',
    '- [Blocked](https://evil.example.net/login)',
    '- https://evil.example.net/raw',
].join('\n');

function buildStreamingReply(content: string): string {
    const chunk = (delta: Record<string, unknown>, finishReason: string | null) => `data: ${JSON.stringify({
        id: 'chatcmpl-links',
        object: 'chat.completion.chunk',
        created: 1708124577,
        model: 'gpt-mock',
        choices: [{ index: 0, delta, finish_reason: finishReason }],
    })}`;
    return [
        chunk({ role: 'assistant', content: '' }, null),
        chunk({ content }, null),
        chunk({}, 'stop'),
        'data: [DONE]',
    ].join('\n\n') + '\n\n';
}

async function screenshotToArtifacts(target: Page | Locator, name: string): Promise<void> {
    const dir = process.env.ARTIFACTS_DIR;
    if (!dir) {
        return;
    }
    await target.screenshot({ path: path.join(dir, name) });
}

let mattermost: MattermostContainer;
let openAIMock: OpenAIMockContainer;

async function setAllowedLinkDomains(value: string): Promise<void> {
    const adminClient = await mattermost.getAdminClient();
    const api = mattermostAIAdminConfigApiFromClient(adminClient, mattermost.url());
    const current = await api.get();
    await api.put({ ...current, allowedLinkDomains: value });
}

async function askForLinks(page: Page): Promise<Locator> {
    const mmPage = new MattermostPage(page);
    const aiPlugin = new AIPlugin(page);
    await mmPage.login(mattermost.url(), username, password);
    await aiPlugin.resetState();

    await openAIMock.addMocks(turnMocksWithTitleSiphon(buildStreamingReply(REPLY_MARKDOWN)));
    await aiPlugin.sendMessage('Give me some links');
    await aiPlugin.waitForBotResponse(REPLY_MARKER);

    const botPost = page.getByTestId('mattermost-ai-rhs').getByTestId('llm-bot-post').last();
    return botPost.getByTestId('posttext').last();
}

test.describe.serial('AI-generated link domain allowlist', () => {
    test.beforeAll(async () => {
        test.setTimeout(240000);
        mattermost = await RunAIMockContainer({
            config: {
                allowUnsafeLinks: true,
                allowedLinkDomains: ALLOWED_LINK_DOMAINS,
            },
        });
        openAIMock = await RunOpenAIMocks(mattermost.network);
    });

    test.afterAll(async () => {
        await openAIMock?.stop();
        await mattermost?.stop();
    });

    test('keeps allowlisted links clickable and renders others as plain text', async ({ page }) => {
        const postText = await askForLinks(page);

        await expect(postText.getByRole('link', { name: 'Allowed', exact: true })).toHaveAttribute('href', 'https://allowed.example.com/page');
        await expect(postText.getByRole('link', { name: 'Sub', exact: true })).toHaveAttribute('href', 'https://docs.trusted.example/x');

        await expect(postText.locator('a[href*="evil.example.net"]')).toHaveCount(0);
        await expect(postText.getByText('Blocked : https://evil.example.net/login', { exact: true })).toBeVisible();
        await expect(postText.getByText('https://evil.example.net/raw', { exact: true })).toBeVisible();

        await screenshotToArtifacts(postText, 'link-allowlist-rendered-post.png');
    });

    test('keeps every link clickable when the allowlist is empty', async ({ page }) => {
        await setAllowedLinkDomains('');

        const postText = await askForLinks(page);

        await expect(postText.getByRole('link', { name: 'Blocked', exact: true })).toHaveAttribute('href', 'https://evil.example.net/login');
        await expect(postText.locator('a[href="https://evil.example.net/raw"]')).toBeVisible();
        await expect(postText.getByRole('link', { name: 'Allowed', exact: true })).toHaveAttribute('href', 'https://allowed.example.com/page');
        await expect(postText).not.toContainText('Blocked : https://evil.example.net/login');

        await screenshotToArtifacts(postText, 'link-allowlist-empty-rendered-post.png');
    });

    test('system console shows and persists the allowlist only while links are rendered', async ({ page }) => {
        await setAllowedLinkDomains('');

        const mmPage = new MattermostPage(page);
        const systemConsole = new SystemConsoleHelper(page);
        await mmPage.login(mattermost.url(), adminUsername, adminPassword);
        await systemConsole.navigateToPluginConfig(mattermost.url());
        await systemConsole.getFunctionsPanel().scrollIntoViewIfNeeded();

        const field = page.getByLabel('Allowed Link Domains (csv)');
        const renderLinksRadios = page.locator('text=Render AI-generated links').locator('..').locator('..').getByRole('radio');
        const trueRadio = renderLinksRadios.first();
        const falseRadio = renderLinksRadios.nth(1);

        await expect(trueRadio).toBeChecked();
        await expect(field).toBeVisible();
        await expect(field).toHaveValue('');

        await field.fill(ALLOWED_LINK_DOMAINS);
        await Promise.all([
            page.waitForResponse((response) => response.request().method() === 'PUT' && response.url().includes('/admin/config')),
            systemConsole.getSaveButton().click(),
        ]);

        await page.reload();
        await systemConsole.getFunctionsPanel().scrollIntoViewIfNeeded();
        await expect(field).toHaveValue(ALLOWED_LINK_DOMAINS);
        await field.scrollIntoViewIfNeeded();
        await page.waitForLoadState('networkidle');
        await screenshotToArtifacts(page, 'link-allowlist-system-console.png');

        await falseRadio.click();
        await expect(field).toBeHidden();
        await trueRadio.click();
        await expect(field).toHaveValue(ALLOWED_LINK_DOMAINS);
    });
});
