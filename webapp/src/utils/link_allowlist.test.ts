// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {isHostAllowed, isLinkAllowed, restrictLinksToAllowedDomains} from './link_allowlist';

const SITE_URL = 'https://mm.example.org';

// Mirrors the anchor markup core's markdown renderer emits for links.
const externalLink = (href: string, text: string) =>
    `<a class="theme markdown__link" href="${href}" rel="noreferrer" target="_blank">${text}</a>`;

describe('isHostAllowed', () => {
    test.each([
        ['exact match', 'example.com', ['example.com'], true],
        ['entries are case-insensitive', 'example.com', ['Example.COM'], true],
        ['hostnames are case-insensitive', 'Example.COM', ['example.com'], true],
        ['plain entry does not match subdomains', 'sub.example.com', ['example.com'], false],
        ['wildcard matches a subdomain', 'sub.example.com', ['*.example.com'], true],
        ['wildcard matches nested subdomains', 'a.b.example.com', ['*.example.com'], true],
        ['wildcard does not match the bare domain', 'example.com', ['*.example.com'], false],
        ['suffix without a dot boundary is rejected', 'evilexample.com', ['*.example.com'], false],
        ['allowed domain as a prefix of another host is rejected', 'example.com.evil.com', ['example.com'], false],
        ['entry written as a URL uses its hostname', 'docs.example.com', ['https://docs.example.com/path'], true],
        ['entry port is ignored', 'example.com', ['example.com:8080'], true],
        ['trailing dot on the host is ignored', 'example.com.', ['example.com'], true],
        ['unicode entries match punycode hosts', 'xn--bcher-kva.example', ['bücher.example'], true],
        ['bare wildcard matches nothing', 'example.com', ['*'], false],
        ['empty wildcard matches nothing', 'example.com', ['*.'], false],
        ['mid-label wildcard matches nothing', 'example.com', ['exa*mple.com'], false],
        ['empty allowlist matches nothing', 'example.com', [], false],
        ['any matching entry is enough', 'docs.example.com', ['other.com', '*.example.com'], true],
    ])('%s', (_name, hostname, allowedDomains, expected) => {
        expect(isHostAllowed(hostname, allowedDomains)).toBe(expected);
    });
});

describe('isLinkAllowed', () => {
    const allowed = ['example.com'];

    test.each([
        ['allowed https host', 'https://example.com/page', SITE_URL, true],
        ['allowed http host', 'http://example.com', SITE_URL, true],
        ['other host', 'https://evil.com', SITE_URL, false],
        ['allowed host as userinfo', 'https://example.com@evil.com/', SITE_URL, false],
        ['protocol-relative other host', '//evil.com/x', SITE_URL, false],
        ['backslash before allowed host', 'https://evil.com\\@example.com', SITE_URL, false],
        ['javascript scheme', 'javascript:alert(1)', SITE_URL, false],
        ['mailto scheme', 'mailto:someone@example.com', SITE_URL, false],
        ['non-http scheme on allowed host', 'ftp://example.com/file', SITE_URL, false],
        ['unparseable URL', 'http://[invalid', SITE_URL, false],
        ['permalink on this server', 'https://mm.example.org/team/pl/abc', SITE_URL, true],
        ['relative link on this server', '/team/channels/town-square', SITE_URL, true],
        ['site URL as a prefix of another host', 'https://mm.example.org.evil.com/', SITE_URL, false],
        ['subpath site: link under the subpath', 'https://host.org/mm/team/pl/abc', 'https://host.org/mm', true],
        ['subpath site: link outside the subpath', 'https://host.org/other', 'https://host.org/mm', false],
        ['subpath site: relative link outside the subpath', '/other', 'https://host.org/mm', false],
        ['subpath site: sibling path sharing a prefix', 'https://host.org/mmevil', 'https://host.org/mm', false],
    ])('%s', (_name, href, siteURL, expected) => {
        expect(isLinkAllowed(href, allowed, siteURL)).toBe(expected);
    });
});

describe('restrictLinksToAllowedDomains', () => {
    const allowed = ['example.com'];

    const render = (html: string) => {
        const container = document.createElement('div');
        container.innerHTML = restrictLinksToAllowedDomains(html, allowed, SITE_URL);
        return container;
    };

    test('returns the HTML unchanged when the allowlist is empty', () => {
        const html = `<p>${externalLink('https://evil.com', 'click')}</p>`;
        expect(restrictLinksToAllowedDomains(html, [], SITE_URL)).toBe(html);
    });

    test('keeps links to allowed domains clickable', () => {
        const container = render(`<p>${externalLink('https://example.com/docs', 'docs')}</p>`);

        const anchor = container.querySelector('a');
        expect(anchor?.getAttribute('href')).toBe('https://example.com/docs');
        expect(anchor?.textContent).toBe('docs');
    });

    test('shows disallowed links as text followed by their destination', () => {
        const container = render(`<p>See ${externalLink('https://evil.com/login', 'the docs')} now</p>`);

        expect(container.querySelector('a')).toBeNull();
        expect(container.textContent).toBe('See the docs : https://evil.com/login now');
    });

    test('does not repeat the destination when the link text is the URL', () => {
        const container = render(`<p>${externalLink('https://evil.com/login', 'https://evil.com/login')}</p>`);

        expect(container.querySelector('a')).toBeNull();
        expect(container.textContent).toBe('https://evil.com/login');
    });

    test('does not repeat the destination for scheme-less autolinks', () => {
        const container = render(`<p>${externalLink('http://www.evil.com', 'www.evil.com')}</p>`);

        expect(container.querySelector('a')).toBeNull();
        expect(container.textContent).toBe('www.evil.com');
    });

    test('preserves formatting inside a disallowed link', () => {
        const container = render(`<p>${externalLink('https://evil.com', '<strong>Click</strong>')}</p>`);

        expect(container.querySelector('a')).toBeNull();
        expect(container.querySelector('strong')?.textContent).toBe('Click');
        expect(container.textContent).toBe('Click : https://evil.com');
    });

    test('renders the destination of a disallowed link as inert text', () => {
        const container = render(`<p>${externalLink('https://evil.com/&lt;img src=x&gt;', 'x')}</p>`);

        expect(container.querySelector('a')).toBeNull();
        expect(container.querySelector('img')).toBeNull();
        expect(container.textContent).toBe('x : https://evil.com/<img src=x>');
    });

    test('filters each link independently', () => {
        const container = render(`<ul><li>${externalLink('https://example.com', 'good')}</li><li>${externalLink('https://evil.com', 'bad')}</li></ul>`);

        const anchors = container.querySelectorAll('a');
        expect(anchors).toHaveLength(1);
        expect(anchors[0].getAttribute('href')).toBe('https://example.com');
        expect(container.textContent).toContain('bad : https://evil.com');
    });

    test('keeps links back into this Mattermost server', () => {
        const container = render('<p><a class="mention-link" href="/team/channels/town-square" data-channel-mention="town-square">~town-square</a></p>');

        expect(container.querySelector('a')?.getAttribute('data-channel-mention')).toBe('town-square');
    });

    test('returns HTML without links unchanged', () => {
        const html = '<p>No <strong>links</strong> here</p>';
        expect(restrictLinksToAllowedDomains(html, allowed, SITE_URL)).toBe(html);
    });
});
