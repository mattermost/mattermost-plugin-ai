import {describe, it} from 'node:test';

import tsParser from '@typescript-eslint/parser';
import {RuleTester} from 'eslint';

import plugin from './index.mjs';

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const tester = new RuleTester({languageOptions: {parser: tsParser}});

tester.run('scoped-tool-label', plugin.rules['scoped-tool-label'], {
    valid: [
        'await expect(toolCard(post, createPostLabel)).toBeVisible();',
        'await expect(post.locator(TOOL_CARD_SELECTOR).getByText(createPostLabel, {exact: true})).toBeVisible();',
        'const card = post.locator(TOOL_CARD_SELECTOR).first(); await card.getByText(createPostLabel).click();',
        'await toolCard(rounds, readPostLabel).getByText(readPostLabel, {exact: true}).click();',
        'await expect(post.getByText(searchToolsLabel, {exact: true})).toHaveCount(0);',
        'await expect(post.getByText(continuationMarker)).toBeVisible();',
        'await expect(rhs.getByRole("button", {name: /accept/i})).toBeVisible();',
    ],
    invalid: [
        {code: 'await expect(post.getByText(createPostLabel, {exact: true})).toBeVisible();', errors: [{messageId: 'unscoped'}]},
        {code: 'const rounds = await expandToolActivity(post); await rounds.getByText(readPostLabel).click();', errors: [{messageId: 'unscoped'}]},
        {code: 'await expect(post.getByRole("button", {name: createPostToolLabel})).toBeVisible();', errors: [{messageId: 'unscoped'}]},
        {code: 'await expect(post.locator("div").filter({hasText: getChannelInfoLabel})).toBeVisible();', errors: [{messageId: 'unscoped'}]},
        {code: 'await expect(post.getByText(loadToolLabel)).toHaveCount(1);', errors: [{messageId: 'unscoped'}]},
    ],
});

tester.run('container-hook-timeout', plugin.rules['container-hook-timeout'], {
    valid: [
        'test.beforeAll(async () => { test.setTimeout(180000); mattermost = await RunContainer(); });',
        'test.describe("x", () => { test.beforeAll(async () => { test.setTimeout(180000); stack = await startAIMockCitationStack({}); }); });',
        'test.beforeAll(async () => { await mattermost.mmctl("config", "set", "A", "b"); });',
        'test.beforeEach(async () => { await RunContainer(); });',
    ],
    invalid: [
        {code: 'test.beforeAll(async () => { mattermost = await RunContainer(); });', errors: [{messageId: 'missing'}]},
        {code: 'test.beforeAll(async () => { mattermost = await RunContainer(); test.setTimeout(180000); });', errors: [{messageId: 'missing'}]},
        {code: 'test.describe("x", () => { test.beforeAll(async () => { if (ready) { mock = await RunOpenAIMocks(net); } }); });', errors: [{messageId: 'missing'}]},
    ],
});

tester.run('container-exec-exit-code', plugin.rules['container-exec-exit-code'], {
    valid: [
        'const {output, exitCode} = await this.container.exec(["mmctl", "--local", "system", "version"]);',
        'const {exitCode} = await mattermost.container.exec(["true"]);',
        'if ((await container.exec(["true"])).exitCode !== 0) { throw new Error("failed"); }',
        'const [, id] = ARTIFACT_URL.exec(src);',
    ],
    invalid: [
        {code: 'await mattermost.container.exec(["mmctl", "--local", "config", "set", "A", "b"]);', errors: [{messageId: 'unchecked'}]},
        {code: 'const {output} = await this.container.exec(["mmctl", "--local", "logs"]);', errors: [{messageId: 'unchecked'}]},
        {code: 'const result = await pgContainer.exec(["psql"]);', errors: [{messageId: 'unchecked'}]},
    ],
});
