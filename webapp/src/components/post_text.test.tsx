// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';

import manifest from '@/manifest';
import {LinkPolicy} from '@/utils/link_allowlist';

import PostText from './post_text';

let mockState: Record<string, unknown> = {};

jest.mock('react-redux', () => ({
    useSelector: (selector: (state: unknown) => unknown) => selector(mockState),
}));

const stateWithLinkPolicy = (linkPolicy?: LinkPolicy) => ({
    entities: {
        channels: {channels: {}},
        teams: {teams: {}},
        general: {config: {SiteURL: 'http://localhost:8065'}},
    },
    ['plugins-' + manifest.id]: linkPolicy ? {linkPolicy} : {},
});

// Mirrors the anchors core's markdown renderer emits: internal links carry
// data-link with the site-relative path, external links open in a new tab.
const renderedHTML = [
    '<a href="http://localhost:8065/_redirect/pl/abc123" data-link="/_redirect/pl/abc123"><strong>internal</strong></a>',
    '<a href="https://example.com/page" target="_blank">external</a>',
].join(' ');

describe('PostText internal links', () => {
    const push = jest.fn();

    beforeEach(() => {
        push.mockClear();
        mockState = stateWithLinkPolicy({kind: 'any_domain'});
        (window as any).PostUtils = {
            formatText: () => renderedHTML,
            messageHtmlToComponent: (html: string) => <span dangerouslySetInnerHTML={{__html: html}}/>,
        };
        window.WebappUtils = {sendWebSocketMessage: jest.fn(), browserHistory: {push}};
    });

    afterEach(() => {
        delete (window as any).PostUtils;
        delete window.WebappUtils;
    });

    test.each([
        {name: 'plain click on internal link navigates in-app', link: 'internal', init: {}, expectedPush: '/_redirect/pl/abc123'},
        {name: 'ctrl-click on internal link keeps browser behavior', link: 'internal', init: {ctrlKey: true}, expectedPush: null},
        {name: 'meta-click on internal link keeps browser behavior', link: 'internal', init: {metaKey: true}, expectedPush: null},
        {name: 'middle-click on internal link keeps browser behavior', link: 'internal', init: {button: 1}, expectedPush: null},
        {name: 'external link keeps browser behavior', link: 'external', init: {}, expectedPush: null},
    ])('$name', ({link, init, expectedPush}) => {
        render(
            <PostText
                message='ignored'
                channelID='channel'
                postID='post'
            />,
        );

        const notPrevented = fireEvent.click(screen.getByText(link), init);

        if (expectedPush) {
            expect(push).toHaveBeenCalledWith(expectedPush);
            expect(notPrevented).toBe(false);
        } else {
            expect(push).not.toHaveBeenCalled();
            expect(notPrevented).toBe(true);
        }
    });
});

describe('PostText link policy', () => {
    const formatText = jest.fn((_message: string, _options: {unsafeLinks: boolean}) => renderedHTML);

    beforeEach(() => {
        formatText.mockClear();
        (window as any).PostUtils = {
            formatText,
            messageHtmlToComponent: (html: string) => <span dangerouslySetInnerHTML={{__html: html}}/>,
        };
    });

    afterEach(() => {
        delete (window as any).PostUtils;
    });

    test.each([
        {name: 'no policy loaded yet renders links as plain text', policy: undefined, unsafeLinks: true, externalClickable: true},
        {name: 'plain text policy renders links as plain text', policy: {kind: 'plain_text'} as LinkPolicy, unsafeLinks: true, externalClickable: true},
        {name: 'any domain policy keeps external links', policy: {kind: 'any_domain'} as LinkPolicy, unsafeLinks: false, externalClickable: true},
        {
            name: 'allowlist keeps external links to allowed domains',
            policy: {kind: 'allowlist', allowedDomains: [{host: 'example.com', wildcard: false}]} as LinkPolicy,
            unsafeLinks: false,
            externalClickable: true,
        },
        {
            name: 'allowlist turns external links to other domains into text',
            policy: {kind: 'allowlist', allowedDomains: [{host: 'other.com', wildcard: false}]} as LinkPolicy,
            unsafeLinks: false,
            externalClickable: false,
        },
    ])('$name', ({policy, unsafeLinks, externalClickable}) => {
        mockState = stateWithLinkPolicy(policy);

        render(
            <PostText
                message='ignored'
                channelID='channel'
                postID='post'
            />,
        );

        // The stubbed formatText ignores its options, so the external anchor
        // only disappears when PostText itself strips it.
        expect(formatText.mock.calls[0][1].unsafeLinks).toBe(unsafeLinks);
        expect(screen.getByText('internal').closest('a')).not.toBeNull();
        expect(screen.queryByRole('link', {name: 'external'}) !== null).toBe(externalClickable);
    });
});
