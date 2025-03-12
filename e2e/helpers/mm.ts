import { Page, Locator, expect } from '@playwright/test';

export class MattermostPage {
    readonly page: Page;
    readonly postTextbox: Locator;
    readonly sendButton: Locator;

    constructor(page: Page) {
        this.page = page;
        this.postTextbox = page.getByTestId('post_textbox');
        this.sendButton = page.getByTestId('channel_view').getByTestId('SendMessageButton');
    }

    async login(url: string, username: string, password: string) {
        await this.page.addInitScript(() => { localStorage.setItem('__landingPageSeen__', 'true'); });
        await this.page.goto(url);
        await this.page.getByText('Log in to your account').waitFor();
        await this.page.getByPlaceholder('Password').fill(password);
        await this.page.getByPlaceholder("Email or Username").fill(username);
        await this.page.getByTestId('saveSetting').click();
    }

    async sendChannelMessage(message: string) {
        await this.postTextbox.click();
        await this.postTextbox.fill(message);
        await this.sendButton.press('Enter');
    }

    async mentionBot(botName: string, message: string) {
        await this.sendChannelMessage(`@${botName} ${message}`);
    }

    async waitForReply() {
        await expect(this.page.getByText('1 reply')).toBeVisible();
    }

    async expectNoReply() {
        await expect(this.page.getByText('reply')).not.toBeVisible();
    }
}

// Legacy function for backward compatibility
export const login = async (page: Page, url: string, username: string, password: string) => {
    const mmPage = new MattermostPage(page);
    await mmPage.login(url, username, password);
};
