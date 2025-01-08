import { test, expect } from '@playwright/test';

import RunContainer from 'helpers/plugincontainer';
import MattermostContainer from 'helpers/mmcontainer';
import {login} from 'helpers/mm';
import {openRHS} from 'helpers/ai-plugin';
import { OpenAIMockContainer, RunOpenAIMocks, responseTest, responseTest2, responseTest2Text, responseTestText } from 'helpers/openai-mock';

let mattermost: MattermostContainer;
let openAIMock: OpenAIMockContainer;

test.beforeAll(async () => {
	test.setTimeout(60000);
	mattermost = await RunContainer();
	openAIMock = await RunOpenAIMocks(mattermost.network)
});

test.afterAll(async () => {
    try {
        if (openAIMock) await openAIMock.stop();
        if (mattermost) await mattermost.stop();
    } catch (error) {
        console.error('Error during cleanup:', error);
    }
}, {timeout: 30000}); // Explicit timeout for cleanup

test('was installed', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openRHS(page);
});


test('rhs bot interaction', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openRHS(page);

	await openAIMock.addCompletionMock(responseTest);
	await page.getByTestId('reply_textbox').fill('Hello!');
	await page.locator('#rhsContainer').getByTestId('SendMessageButton').click()
	await expect(page.getByText("Hello! How can I assist you today?")).toBeVisible();
})

test('rhs prompt templates', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openRHS(page);

	// Clicking prompt template adds message
	await page.getByRole('button', { name: 'Brainstorm ideas' }).click();
	await expect(page.getByTestId('reply_textbox')).toHaveText("Brainstorm ideas about ");

	// Clicking without editing replaces the text
	await page.getByRole('button', { name: 'To-do list' }).click();
	await expect(page.getByTestId('reply_textbox')).toHaveText("Write a todo list about ");

	// If text has been edited, clicking will not replace the text
	/*await page.getByTestId('reply_textbox').fill('Edited text');
	await page.getByRole('button', { name: 'Pros and Cons' }).click();
	await expect(page.getByTestId('reply_textbox')).toHaveText("Edited text");*/
})

test ('regenerate button', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openRHS(page);
	await openAIMock.addCompletionMock(responseTest);

	await page.getByTestId('reply_textbox').fill('Hello!');
	await page.locator('#rhsContainer').getByTestId('SendMessageButton').click()
	await expect(page.getByText(responseTestText)).toBeVisible();

	await openAIMock.addCompletionMock(responseTest2);

	await page.getByRole('button', { name: 'Regenerate' }).click();
	await expect(page.getByText(responseTest2Text)).toBeVisible();
})

test ('switching bots', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openRHS(page);
	await openAIMock.addCompletionMock(responseTest, "second");

	// Switch to second bot
	await page.getByTestId('menuButtonMock Bot').click();
	await page.getByRole('button', { name: 'Second Bot' }).click();

	await page.getByTestId('reply_textbox').fill('Hello!');
	await page.locator('#rhsContainer').getByTestId('SendMessageButton').click()

	// Second bot responds
	await expect(page.getByRole('button', { name: 'second', exact: true })).toBeVisible();
	// With correct message
	await expect(page.getByText(responseTestText)).toBeVisible();
})

test('bot mention', async ({ page }) => {
	const url = mattermost.url()
	await login(page, url, "regularuser", "regularuser");;
	await openAIMock.addCompletionMock(responseTest);

	await page.getByTestId('post_textbox').click();
	await page.getByTestId('post_textbox').fill('`@mock` TestBotMention1');
	await page.getByTestId('post_textbox').press('Enter');
	await expect(page.getByText("reply")).not.toBeVisible();

	await page.getByTestId('post_textbox').fill('```\n@mock\n``` TestBotMention2');
	await page.getByTestId('post_textbox').press('Enter');
	await expect(page.getByText("reply")).not.toBeVisible();

	await page.getByTestId('post_textbox').fill('@mock TestBotMention3');
	await page.getByTestId('post_textbox').press('Enter');
	await expect(page.getByText("1 reply")).toBeVisible();
})
