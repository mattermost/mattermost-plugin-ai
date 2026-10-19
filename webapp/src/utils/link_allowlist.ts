// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

type DomainRule = {
    host: string;
    wildcard: boolean;
};

const WILDCARD_PREFIX = '*.';

function normalizeHostname(hostname: string): string {
    return hostname.toLowerCase().replace(/\.$/, '');
}

// Entries are parsed with the URL parser so they compare in the same
// canonical form (punycode, no port or path) as the link being checked.
function parseRule(entry: string): DomainRule | null {
    let value = entry.trim().toLowerCase();
    if (value.includes('://')) {
        try {
            value = new URL(value).hostname;
        } catch {
            return null;
        }
    }

    const wildcard = value.startsWith(WILDCARD_PREFIX);
    if (wildcard) {
        value = value.slice(WILDCARD_PREFIX.length);
    }
    if (!value || value.includes('*')) {
        return null;
    }

    try {
        const host = normalizeHostname(new URL(`http://${value}`).hostname);
        return host ? {host, wildcard} : null;
    } catch {
        return null;
    }
}

/**
 * Reports whether hostname is covered by the allowlist. A plain entry
 * ("example.com") matches only that host; a wildcard entry ("*.example.com")
 * matches any subdomain of it but not the bare domain.
 */
export function isHostAllowed(hostname: string, allowedDomains: string[]): boolean {
    const host = normalizeHostname(hostname);
    if (!host) {
        return false;
    }
    return allowedDomains.some((entry) => {
        const rule = parseRule(entry);
        if (!rule) {
            return false;
        }
        return rule.wildcard ? host.endsWith(`.${rule.host}`) : host === rule.host;
    });
}

function isInternalURL(url: URL, siteRoot: string): boolean {
    return url.href === siteRoot || ['/', '?', '#'].some((sep) => url.href.startsWith(siteRoot + sep));
}

/**
 * Reports whether a link may stay clickable. Links back into this Mattermost
 * server are always allowed; any other link must be http(s) to an allowed host.
 */
export function isLinkAllowed(href: string, allowedDomains: string[], siteURL: string): boolean {
    let url: URL;
    let siteRoot: string;
    try {
        siteRoot = new URL(siteURL).href.replace(/\/$/, '');
        url = new URL(href, siteRoot + '/');
    } catch {
        return false;
    }

    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
        return false;
    }
    return isInternalURL(url, siteRoot) || isHostAllowed(url.hostname, allowedDomains);
}

function isRedundantHref(text: string, href: string): boolean {
    return href === text || href === `http://${text}` || href === `mailto:${text}`;
}

/**
 * Turns every link in rendered post HTML whose destination is not allowed into
 * plain text, keeping the destination visible the way core does for unsafe
 * links ("text : href"). An empty allowlist leaves the HTML untouched.
 */
export function restrictLinksToAllowedDomains(html: string, allowedDomains: string[], siteURL?: string): string {
    if (allowedDomains.length === 0 || !html) {
        return html;
    }

    // A template's content is inert, so parsing never loads images or runs scripts.
    const template = document.createElement('template');
    template.innerHTML = html;

    const base = siteURL || window.location.origin;
    let changed = false;
    template.content.querySelectorAll('a[href]').forEach((anchor) => {
        const href = anchor.getAttribute('href') ?? '';
        if (isLinkAllowed(href, allowedDomains, base)) {
            return;
        }

        const text = anchor.textContent ?? '';
        const replacement = document.createDocumentFragment();
        replacement.append(...Array.from(anchor.childNodes));
        if (!isRedundantHref(text, href)) {
            replacement.append(` : ${href}`);
        }
        anchor.replaceWith(replacement);
        changed = true;
    });

    return changed ? template.innerHTML : html;
}
