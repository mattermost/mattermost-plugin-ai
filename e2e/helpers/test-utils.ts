import { expect, test as baseTest } from '@playwright/test';
import { AIPlugin } from './ai-plugin';
import { MattermostPage } from './mm';
import { OpenAIMockContainer } from './openai-mock';
import MattermostContainer from './mmcontainer';

// Create a custom test fixture
export const test = baseTest.extend({
  mattermostPage: async ({ page }, use) => {
    // Setup the MM page
    const mmPage = new MattermostPage(page);
    await use(mmPage);
  },
  
  aiPlugin: async ({ page }, use) => {
    // Setup the AI plugin
    const aiPlugin = new AIPlugin(page);
    await use(aiPlugin);
  },
});

// Custom assertions and helpers
export const expectResponseWithinTimeout = async (aiPlugin: AIPlugin, expectedText: string, timeoutMs = 10000) => {
  await expect(
    async () => {
      const isVisible = await aiPlugin.page.getByText(expectedText).isVisible();
      expect(isVisible).toBe(true);
    },
    {
      message: `Response "${expectedText}" not received within ${timeoutMs}ms`,
      timeout: timeoutMs
    }
  ).toPass();
};

// Type definitions for test data
export interface TestUser {
  username: string;
  password: string;
  email: string;
}

export interface TestBot {
  name: string;
  displayName: string;
  service: {
    type: string;
    apiKey: string;
    apiURL: string;
  };
}

// Parametrized test helpers
export const testWithDifferentUsers = (users: TestUser[], testFn: (user: TestUser) => Promise<void>) => {
  for (const user of users) {
    test(`with user ${user.username}`, async () => {
      await testFn(user);
    });
  }
};

// Screenshot helpers
export const takeScreenshotOnFailure = async (testInfo, page) => {
  if (testInfo.status !== 'passed') {
    await page.screenshot({ path: `test-results/failures/${testInfo.title.replace(/\s+/g, '-')}.png`, fullPage: true });
  }
};