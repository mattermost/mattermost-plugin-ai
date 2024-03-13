import { Page } from 'playwright';
import { expect } from '@playwright/test';

export const openRHS = async (page: Page) => {
	await page.locator('#app-bar-icon-mattermost-ai').click();
	await expect(page.locator('#app-bar-icon-mattermost-ai')).toBeVisible();
};
