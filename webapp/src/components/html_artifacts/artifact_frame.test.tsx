// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {Provider} from 'react-redux';
import {createStore} from 'redux';

import {getArtifactToken} from '@/client';

import {resetConsentStore} from './artifact_broker';
import ArtifactConsentPrompt, {CONSENT_ARM_DELAY_MS} from './artifact_consent_prompt';
import ArtifactFrame, {HEARTBEAT_HIDE_MS, HEARTBEAT_STOP_MS} from './artifact_frame';
import HTMLArtifactCard, {resetArtifactRunDecisions} from './html_artifact_card';
import HTMLArtifactFullscreen from './html_artifact_fullscreen';
import {resetArtifactTokenCache} from './artifact_token';

jest.mock('@/client', () => ({
    artifactURL: (fileId: string) => `/plugins/mattermost-ai/artifacts/${fileId}`,
    fileDownloadURL: (fileId: string) => `/api/v4/files/${fileId}?download=1`,
    getArtifactToken: jest.fn(),
}));

const mockGetToken = getArtifactToken as jest.MockedFunction<typeof getArtifactToken>;
const TOKEN = 'bridge-token';

// Lets the token fetch resolve and the frame mount.
const flushToken = () => act(async () => {
    await Promise.resolve();
});

// Messages carry no ids until the build extracts them; render defaultMessage.
jest.mock('react-intl', () => {
    const actual = jest.requireActual('react-intl');
    const format = ({defaultMessage}: {defaultMessage: string}, values: Record<string, string> = {}) =>
        defaultMessage.replace(/\{(\w+)\}/g, (_: string, k: string) => values[k] ?? '');
    return {
        ...actual,
        useIntl: () => ({locale: 'en', formatMessage: format}),
        FormattedMessage: (props: {defaultMessage: string; values?: Record<string, string>}) => format(props, props.values),
    };
});

const store = createStore(() => ({
    entities: {
        preferences: {myPreferences: {}},
        users: {currentUserId: 'u1', profiles: {u1: {id: 'u1', username: 'alice'}}},
    },
}));

function withProviders(children: React.ReactNode) {
    return (
        <Provider store={store}>
            {children}
        </Provider>
    );
}

beforeEach(() => {
    jest.useFakeTimers();
    resetConsentStore();
    resetArtifactTokenCache();
    resetArtifactRunDecisions();
    mockGetToken.mockReset();
    mockGetToken.mockResolvedValue({token: TOKEN, tooLarge: false});
});

afterEach(() => {
    jest.useRealTimers();
});

const advance = (ms: number) => act(() => {
    jest.advanceTimersByTime(ms);
});

// jsdom marks every event dispatched from script as untrusted. Dispatching
// through its internal implementation stands in for a real user's click.
type JsdomImpl = {isTrusted: boolean; _dispatch: (event: JsdomImpl) => boolean};
function jsdomImpl(wrapper: object): JsdomImpl {
    const symbol = Object.getOwnPropertySymbols(wrapper).find((s) => s.description === 'impl');
    return (wrapper as Record<symbol, JsdomImpl>)[symbol!];
}
function trustedClick(el: Element) {
    const event = jsdomImpl(new MouseEvent('click', {bubbles: true, cancelable: true}));
    event.isTrusted = true;
    act(() => {
        jsdomImpl(el)._dispatch(event); // eslint-disable-line no-underscore-dangle
    });
}

// A host element outside every artifact, standing in for the message box.
function addComposer(): HTMLTextAreaElement {
    const composer = document.createElement('textarea');
    document.body.appendChild(composer);
    return composer;
}

const frameIframe = () => screen.queryByTitle(/Interactive artifact/) as HTMLIFrameElement | null;

// Renders an inline frame and walks it through the host load and the bridge's ready.
async function renderReadyFrame(props: Partial<React.ComponentProps<typeof ArtifactFrame>> = {}) {
    const result = render(withProviders(
        <ArtifactFrame
            fileId='abcdefghijklmnopqrstuvwxyz'
            fileName='guard.html'
            displayMode='inline'
            {...props}
        />,
    ));
    await flushToken();
    const frame = frameIframe()!;
    const win = frame.contentWindow!;
    const send = (data: Record<string, unknown>) => act(() => {
        window.dispatchEvent(new MessageEvent('message', {data: {...data, token: TOKEN}, origin: 'null', source: win}));
    });
    fireEvent.load(frame);
    send({mmArtifact: 1, type: 'ready'});
    return {...result, frame, win, send};
}

const stopped = () => screen.queryByTestId('html-artifact-navigation-stopped');

describe('ArtifactFrame navigation guard', () => {
    const iframe = () => screen.queryByTitle(/Interactive artifact/) as HTMLIFrameElement | null;

    test('unloads a self-navigating artifact without remounting until the viewer reloads', async () => {
        render(withProviders(
            <ArtifactFrame
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='nav.html'
                displayMode='inline'
            />,
        ));

        await flushToken();

        // Host load, then the artifact navigates: the frame is unloaded, never remounted.
        const first = iframe()!;
        fireEvent.load(first);
        expect(iframe()).toBe(first);
        fireEvent.load(first);
        expect(iframe()).toBeNull();
        expect(screen.getByTestId('html-artifact-navigation-stopped').textContent).toContain('tried to navigate away');

        // A manual reload mounts a fresh frame, guarded the same way.
        fireEvent.click(screen.getByRole('button', {name: 'Reload'}));
        await flushToken();
        const second = iframe()!;
        expect(second).not.toBeNull();
        expect(second).not.toBe(first);
        fireEvent.load(second);
        fireEvent.load(second);
        expect(iframe()).toBeNull();
    });

    test('stops answering the frame and drops a pending consent prompt on a second load', async () => {
        render(withProviders(
            <ArtifactFrame
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='nav.html'
                displayMode='inline'
                initiallyEngaged={true}
            />,
        ));
        await flushToken();
        const frame = iframe()!;
        const win = frame.contentWindow!;
        const posted = jest.spyOn(win, 'postMessage');
        const send = (data: unknown) => act(() => {
            window.dispatchEvent(new MessageEvent('message', {data: {...(data as object), token: TOKEN}, origin: 'null', source: win}));
        });

        fireEvent.load(frame);
        send({mmArtifact: 1, type: 'ready'});
        send({mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser'});
        expect(screen.getByTestId('html-artifact-consent-deny')).toBeTruthy();

        fireEvent.load(frame);
        expect(screen.queryByTestId('html-artifact-consent-deny')).toBeNull();

        // The replacement document gets no answers, even to a fresh ready/request.
        posted.mockClear();
        send({mmArtifact: 1, type: 'ready'});
        send({mmArtifact: 1, type: 'request', id: 'r2', method: 'getCurrentUser'});
        expect(posted).not.toHaveBeenCalled();
    });
});

describe('ArtifactFrame heartbeat watchdog', () => {
    const unresponsive = () => screen.queryByTestId('html-artifact-unresponsive');
    const heartbeat = {mmArtifact: 1, type: 'heartbeat'};

    let visibility: DocumentVisibilityState = 'visible';
    beforeEach(() => {
        visibility = 'visible';
        Object.defineProperty(document, 'visibilityState', {configurable: true, get: () => visibility});
    });
    afterEach(() => {
        delete (document as {visibilityState?: unknown}).visibilityState;
    });
    const setVisibility = (state: DocumentVisibilityState) => act(() => {
        visibility = state;
        document.dispatchEvent(new Event('visibilitychange'));
    });

    test('hides a frame whose heartbeat lapses, and shows it again when the heartbeat resumes', async () => {
        const {frame, send} = await renderReadyFrame();
        send(heartbeat);
        advance(HEARTBEAT_HIDE_MS - 250);
        expect(unresponsive()).toBeNull();
        expect(window.getComputedStyle(frame).visibility).toBe('visible');

        advance(500);
        expect(unresponsive()?.textContent).toContain('stopped responding');
        expect(window.getComputedStyle(frame).visibility).toBe('hidden');
        expect(frameIframe()).toBe(frame);

        send(heartbeat);
        expect(unresponsive()).toBeNull();
        expect(window.getComputedStyle(frame).visibility).toBe('visible');
        expect(stopped()).toBeNull();
    });

    test('unloads a frame whose heartbeat stays gone, dropping its prompt and answering nothing more', async () => {
        const {frame, win, send} = await renderReadyFrame({initiallyEngaged: true});
        send(heartbeat);
        send({mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser'});
        expect(screen.getByTestId('html-artifact-consent')).toBeTruthy();

        const posted = jest.spyOn(win, 'postMessage');
        advance(HEARTBEAT_STOP_MS + 500);
        expect(frameIframe()).toBeNull();
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('unresponsive');
        expect(stopped()?.textContent).toContain('stopped responding and was unloaded');
        expect(screen.queryByTestId('html-artifact-consent')).toBeNull();

        posted.mockClear();
        send({mmArtifact: 1, type: 'ready'});
        send({mmArtifact: 1, type: 'request', id: 'r2', method: 'getCurrentUser'});
        expect(posted).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', {name: 'Reload'}));
        await flushToken();
        expect(frameIframe()).not.toBeNull();
        expect(frameIframe()).not.toBe(frame);
        expect(stopped()).toBeNull();
    });

    test('does not watch a frame before its first heartbeat', async () => {
        await renderReadyFrame();
        advance(HEARTBEAT_STOP_MS * 3);
        expect(unresponsive()).toBeNull();
        expect(stopped()).toBeNull();
        expect(frameIframe()).not.toBeNull();
    });

    // The bridge's first heartbeat goes out before a parser-blocking
    // location.replace() swaps the document: that frame never gets ready,
    // so the ready timeout unloads it and offers a retry.
    test('unloads a frame that never got ready when the ready timeout fires, and retry remounts it', async () => {
        render(withProviders(
            <ArtifactFrame
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='replaced.html'
                displayMode='inline'
            />,
        ));
        await flushToken();
        const frame = frameIframe()!;
        act(() => {
            window.dispatchEvent(new MessageEvent('message', {data: {...heartbeat, token: TOKEN}, origin: 'null', source: frame.contentWindow}));
        });
        fireEvent.load(frame);
        advance(HEARTBEAT_STOP_MS + 500);
        expect(stopped()).toBeNull();
        expect(frameIframe()).toBe(frame);
        expect(window.getComputedStyle(frame).visibility).toBe('hidden');
        advance(10000);
        expect(frameIframe()).toBeNull();
        expect(screen.getByRole('alert').textContent).toContain('could not be loaded');
        expect(stopped()).toBeNull();

        fireEvent.click(screen.getByRole('button', {name: 'Retry'}));
        await flushToken();
        expect(frameIframe()).not.toBeNull();
        expect(frameIframe()).not.toBe(frame);
    });

    test('a late check with a stale heartbeat hides the frame but does not unload it', async () => {
        let t = 1000;
        const now = jest.spyOn(performance, 'now').mockImplementation(() => t);
        try {
            const {frame, send} = await renderReadyFrame();
            send(heartbeat);

            t += HEARTBEAT_STOP_MS + 1000;
            advance(250);
            expect(unresponsive()).not.toBeNull();
            expect(window.getComputedStyle(frame).visibility).toBe('hidden');
            expect(stopped()).toBeNull();
            expect(frameIframe()).toBe(frame);

            t += HEARTBEAT_STOP_MS / 2;
            advance(250);
            expect(stopped()).toBeNull();

            send(heartbeat);
            expect(unresponsive()).toBeNull();
            expect(window.getComputedStyle(frame).visibility).toBe('visible');

            for (let elapsed = 0; elapsed <= HEARTBEAT_STOP_MS + 500; elapsed += 250) {
                t += 250;
                advance(250);
            }
            expect(stopped()?.getAttribute('data-stop-reason')).toBe('unresponsive');
        } finally {
            now.mockRestore();
        }
    });

    test('pauses while the page is hidden and grants a fresh grace period once visible', async () => {
        const {send} = await renderReadyFrame();
        send(heartbeat);
        setVisibility('hidden');
        advance(HEARTBEAT_STOP_MS * 10);
        expect(stopped()).toBeNull();
        expect(unresponsive()).toBeNull();

        setVisibility('visible');
        advance(HEARTBEAT_HIDE_MS - 250);
        expect(unresponsive()).toBeNull();
        advance(HEARTBEAT_STOP_MS);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('unresponsive');
    });

    test('a frame stopped by its heartbeat stays down without the viewer reloading', async () => {
        const {send} = await renderReadyFrame();
        send(heartbeat);
        advance(HEARTBEAT_STOP_MS + 500);
        expect(frameIframe()).toBeNull();
        send(heartbeat);
        advance(HEARTBEAT_STOP_MS);
        expect(frameIframe()).toBeNull();
        expect(mockGetToken).toHaveBeenCalledTimes(1);
    });
});

describe('ArtifactFrame focus guard', () => {
    const interact = () => screen.queryByTestId('html-artifact-interact');
    let composer: HTMLTextAreaElement;
    beforeEach(() => {
        composer = addComposer();
    });
    afterEach(() => {
        composer.remove();
    });

    test('covers an artifact the viewer has not engaged; only a trusted click engages it', async () => {
        const {frame} = await renderReadyFrame();
        expect(interact()).not.toBeNull();
        expect(interact()!.tagName).toBe('BUTTON');
        expect(interact()!.getAttribute('aria-label')).toBe('Interact with this artifact');
        expect(frame.getAttribute('tabindex')).toBe('-1');
        expect(frame.hasAttribute('inert')).toBe(false);

        fireEvent.click(interact()!);
        expect(interact()).not.toBeNull();

        trustedClick(interact()!);
        expect(interact()).toBeNull();
        expect(frame.hasAttribute('tabindex')).toBe(false);
        expect(document.activeElement).toBe(frame);
        advance(1000);
        expect(stopped()).toBeNull();
    });

    test.each([
        {name: 'focused before the artifact mounted', focusBeforeMount: true},
        {name: 'focused after the artifact mounted', focusBeforeMount: false},
    ])('stops an artifact that takes focus unasked and returns it to the page element $name', async ({focusBeforeMount}) => {
        if (focusBeforeMount) {
            composer.focus();
        }
        const {frame} = await renderReadyFrame();
        if (!focusBeforeMount) {
            composer.focus();
        }

        act(() => frame.focus());
        advance(300);
        expect(frameIframe()).toBeNull();
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
        expect(stopped()?.textContent).toContain('tried to take keyboard focus');
        expect(document.activeElement).toBe(composer);
    });

    test('an engaged artifact may hold focus until the viewer focuses the page again', async () => {
        const {frame} = await renderReadyFrame({initiallyEngaged: true});
        expect(interact()).toBeNull();
        act(() => frame.focus());
        advance(1000);
        expect(stopped()).toBeNull();

        act(() => composer.focus());
        expect(interact()).not.toBeNull();
        expect(frame.getAttribute('tabindex')).toBe('-1');

        act(() => frame.focus());
        advance(300);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
        expect(document.activeElement).toBe(composer);
    });

    test('a viewer reload starts over as initially engaged', async () => {
        const {frame} = await renderReadyFrame({initiallyEngaged: true});
        act(() => composer.focus());
        act(() => frame.focus());
        advance(300);
        expect(stopped()).not.toBeNull();

        fireEvent.click(screen.getByRole('button', {name: 'Reload'}));
        await flushToken();
        expect(frameIframe()!.hasAttribute('tabindex')).toBe(false);
    });

    test('a click on page content that takes no focus ends engagement without stopping the artifact', async () => {
        const {frame} = await renderReadyFrame({initiallyEngaged: true});
        act(() => frame.focus());
        expect(document.activeElement).toBe(frame);

        act(() => {
            composer.dispatchEvent(new MouseEvent('pointerdown', {bubbles: true}));
        });
        expect(document.activeElement).not.toBe(frame);
        expect(interact()).not.toBeNull();
        advance(1000);
        expect(stopped()).toBeNull();

        act(() => frame.focus());
        advance(300);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
    });

    test('a pointerdown inside the frame keeps it engaged', async () => {
        const {frame} = await renderReadyFrame({initiallyEngaged: true});
        act(() => {
            frame.dispatchEvent(new MouseEvent('pointerdown', {bubbles: true}));
        });
        expect(interact()).toBeNull();
    });

    test('an artifact the viewer has not engaged cannot open the consent prompt', async () => {
        const {win, send} = await renderReadyFrame();
        const posted = jest.spyOn(win, 'postMessage');
        send({mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser'});
        expect(screen.queryByTestId('html-artifact-consent')).toBeNull();
        expect(posted).toHaveBeenCalledWith(expect.objectContaining({id: 'r1', error: expect.objectContaining({code: 'busy'})}), '*');

        trustedClick(interact()!);
        send({mmArtifact: 1, type: 'request', id: 'r2', method: 'getCurrentUser'});
        expect(screen.getByTestId('html-artifact-consent')).toBeTruthy();
    });

    const answerPrompt = () => {
        act(() => {
            jest.advanceTimersByTime(CONSENT_ARM_DELAY_MS);
        });
        const allow = screen.getByTestId('html-artifact-consent-allow');
        act(() => allow.focus());
        fireEvent.click(allow);
        expect(screen.queryByTestId('html-artifact-consent')).toBeNull();
    };

    test('answering the consent prompt hands focus back to the engaged artifact without tripping the guard', async () => {
        const {frame, send} = await renderReadyFrame({initiallyEngaged: true});
        act(() => frame.focus());
        send({mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser'});
        expect(document.activeElement).toBe(screen.getByTestId('html-artifact-consent-deny'));

        // The viewer goes back to the message box before answering.
        act(() => composer.focus());
        answerPrompt();
        expect(document.activeElement).toBe(frame);
        advance(1000);
        expect(stopped()).toBeNull();
        expect(frameIframe()).toBe(frame);
    });

    test('answering the consent prompt does not re-engage a frame hidden by a heartbeat lapse', async () => {
        const {frame, send} = await renderReadyFrame({initiallyEngaged: true});
        act(() => frame.focus());
        send({mmArtifact: 1, type: 'heartbeat'});
        send({mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser'});
        expect(document.activeElement).toBe(screen.getByTestId('html-artifact-consent-deny'));

        advance(HEARTBEAT_HIDE_MS + 500);
        expect(screen.getByTestId('html-artifact-unresponsive')).toBeTruthy();
        expect(screen.getByTestId('html-artifact-consent')).toBeTruthy();

        answerPrompt();
        expect(screen.getByTestId('html-artifact-frame').getAttribute('data-engaged')).toBe('false');
        advance(300);
        expect(screen.getByTestId('html-artifact-frame').getAttribute('data-engaged')).toBe('false');
        expect(frameIframe() !== null && document.activeElement === frameIframe()).toBe(false);

        // The prompt hands focus back to the hidden frame it opened over; the
        // guard stops a disengaged frame holding it.
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
    });

    test('an artifact that takes focus and opens a prompt at once gets no prompt and is stopped', async () => {
        const {frame} = await renderReadyFrame();
        act(() => composer.focus());
        act(() => {
            frame.focus();
            window.dispatchEvent(new MessageEvent('message', {
                data: {mmArtifact: 1, type: 'request', id: 'r1', method: 'getCurrentUser', token: TOKEN},
                origin: 'null',
                source: frame.contentWindow,
            }));
        });
        expect(screen.queryByTestId('html-artifact-consent')).toBeNull();
        advance(300);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
        expect(document.activeElement).toBe(composer);
    });

    test('a frame that lost its heartbeat gives keyboard focus back to the page', async () => {
        const {frame, send} = await renderReadyFrame();
        act(() => composer.focus());
        trustedClick(interact()!);
        expect(document.activeElement).toBe(frame);
        send({mmArtifact: 1, type: 'heartbeat'});
        advance(HEARTBEAT_HIDE_MS + 500);
        expect(screen.getByTestId('html-artifact-unresponsive')).toBeTruthy();
        expect(document.activeElement).toBe(composer);
        expect(stopped()).toBeNull();
    });
});

describe('ArtifactFrame bridge token', () => {
    const iframe = () => screen.queryByTitle(/Interactive artifact/) as HTMLIFrameElement | null;

    function renderFrame() {
        return render(withProviders(
            <ArtifactFrame
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='tok.html'
                displayMode='inline'
            />,
        ));
    }

    test('shows loading and no frame until the token arrives', async () => {
        let resolve: (t: {token: string; tooLarge: boolean}) => void = () => {}; // eslint-disable-line no-empty-function
        mockGetToken.mockReturnValue(new Promise((r) => {
            resolve = r;
        }));
        renderFrame();
        expect(iframe()).toBeNull();
        expect(screen.getByLabelText('Loading artifact')).toBeTruthy();
        await act(async () => {
            resolve({token: TOKEN, tooLarge: false});
        });
        expect(iframe()).not.toBeNull();
    });

    test('shows a download prompt instead of loading a too-large artifact', async () => {
        mockGetToken.mockResolvedValueOnce({token: TOKEN, tooLarge: true});
        renderFrame();
        await flushToken();
        expect(iframe()).toBeNull();
        expect(screen.queryByLabelText('Loading artifact')).toBeNull();
        expect(screen.getByRole('alert').textContent).toContain('too large to preview');
        expect(screen.getByRole('button', {name: 'Download'})).toBeTruthy();
    });

    test('shows the error state on token failure and retries', async () => {
        mockGetToken.mockRejectedValueOnce(new Error('nope'));
        renderFrame();
        await flushToken();
        expect(iframe()).toBeNull();
        expect(screen.getByRole('alert').textContent).toContain('could not be loaded');
        fireEvent.click(screen.getByRole('button', {name: 'Retry'}));
        await flushToken();
        expect(iframe()).not.toBeNull();
        expect(mockGetToken).toHaveBeenCalledTimes(2);
    });

    test('answers only messages carrying the token', async () => {
        renderFrame();
        await flushToken();
        const frame = iframe()!;
        fireEvent.load(frame);
        const posted = jest.spyOn(frame.contentWindow!, 'postMessage');
        const send = (data: unknown) => act(() => {
            window.dispatchEvent(new MessageEvent('message', {data, origin: 'null', source: frame.contentWindow}));
        });
        send({mmArtifact: 1, type: 'ready'});
        send({mmArtifact: 1, type: 'ready', token: 'guess'});
        expect(posted).not.toHaveBeenCalled();
        send({mmArtifact: 1, type: 'ready', token: TOKEN});
        expect(posted).toHaveBeenCalledTimes(1);
    });
});

describe('ArtifactConsentPrompt activation guard', () => {
    function renderPrompt() {
        const onAllow = jest.fn();
        const onDeny = jest.fn();
        render(withProviders(
            <ArtifactConsentPrompt
                fileName='dash.html'
                onAllow={onAllow}
                onDeny={onDeny}
            />,
        ));
        return {
            onAllow,
            onDeny,
            allow: screen.getByTestId('html-artifact-consent-allow'),
            deny: screen.getByTestId('html-artifact-consent-deny'),
        };
    }

    const arm = () => act(() => {
        jest.advanceTimersByTime(CONSENT_ARM_DELAY_MS);
    });

    test('restores the previously focused element when it goes away', () => {
        const outside = document.createElement('button');
        document.body.appendChild(outside);
        outside.focus();
        const {unmount} = render(withProviders(
            <ArtifactConsentPrompt
                fileName='dash.html'
                onAllow={jest.fn()}
                onDeny={jest.fn()}
            />,
        ));
        expect(document.activeElement).toBe(screen.getByTestId('html-artifact-consent-deny'));
        unmount();
        expect(document.activeElement).toBe(outside);
        outside.remove();
    });

    test('leaves focus alone when it moved out of the prompt before unmount', () => {
        const before = document.createElement('button');
        const elsewhere = document.createElement('button');
        document.body.append(before, elsewhere);
        before.focus();
        const {unmount} = render(withProviders(
            <ArtifactConsentPrompt
                fileName='dash.html'
                onAllow={jest.fn()}
                onDeny={jest.fn()}
            />,
        ));
        expect(document.activeElement).toBe(screen.getByTestId('html-artifact-consent-deny'));
        elsewhere.focus();
        unmount();
        expect(document.activeElement).toBe(elsewhere);
        before.remove();
        elsewhere.remove();
    });

    test('shows a heading and focuses Deny', () => {
        const {deny} = renderPrompt();
        expect(screen.getByRole('alertdialog', {name: 'Allow access to your profile?'})).toBeTruthy();
        expect(document.activeElement).toBe(deny);
    });

    const cases: Array<{name: string; armed: boolean; pointerDownAfterMount: boolean; detail: number; expectCalled: boolean}> = [
        {name: 'pointer click right after the prompt appears is ignored', armed: false, pointerDownAfterMount: true, detail: 1, expectCalled: false},
        {name: 'keyboard activation right after the prompt appears is ignored', armed: false, pointerDownAfterMount: false, detail: 0, expectCalled: false},
        {name: 'pointer click whose press started before the prompt is ignored', armed: true, pointerDownAfterMount: false, detail: 1, expectCalled: false},
        {name: 'pointer click after the delay counts', armed: true, pointerDownAfterMount: true, detail: 1, expectCalled: true},
        {name: 'keyboard activation after the delay counts', armed: true, pointerDownAfterMount: false, detail: 0, expectCalled: true},
    ];

    for (const button of ['allow', 'deny'] as const) {
        for (const c of cases) {
            test(`${button}: ${c.name}`, () => {
                const prompt = renderPrompt();
                const el = prompt[button];
                if (c.armed) {
                    arm();
                }
                if (c.pointerDownAfterMount) {
                    if (c.armed) {
                        fireEvent.pointerEnter(el);
                    }
                    fireEvent.pointerDown(el);
                }
                fireEvent.click(el, {detail: c.detail});
                const handler = button === 'allow' ? prompt.onAllow : prompt.onDeny;
                expect(handler).toHaveBeenCalledTimes(c.expectCalled ? 1 : 0);
            });
        }
    }

    test('buttons read as disabled until the delay passes', () => {
        const {allow, deny} = renderPrompt();
        expect(allow.getAttribute('aria-disabled')).toBe('true');
        expect(deny.getAttribute('aria-disabled')).toBe('true');
        arm();
        expect(allow.getAttribute('aria-disabled')).toBe('false');
        expect(deny.getAttribute('aria-disabled')).toBe('false');
    });
});

describe('HTMLArtifactFullscreen focus sentinels', () => {
    // jsdom has no layout: report a box for every element.
    let rects: jest.SpyInstance;
    beforeEach(() => {
        rects = jest.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}] as unknown as DOMRectList);
    });
    afterEach(() => rects.mockRestore());

    // The artifact starts covered (Close takes focus on open), so once it is
    // ready the interact overlay stands in for the iframe as the tab stop.
    test.each([
        {name: 'end sentinel wraps to the first control', sentinel: 'html-artifact-fullscreen-sentinel-end', ready: true, expected: 'Download'},
        {name: 'start sentinel wraps to the interact overlay', sentinel: 'html-artifact-fullscreen-sentinel-start', ready: true, expected: 'Interact with this artifact'},
        {name: 'start sentinel skips the hidden loading iframe', sentinel: 'html-artifact-fullscreen-sentinel-start', ready: false, expected: 'Close'},
    ])('$name', async ({sentinel, ready, expected}) => {
        render(withProviders(
            <HTMLArtifactFullscreen
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='dash.html'
                onClose={jest.fn()}
            />,
        ));
        await flushToken();
        const frame = screen.getByTitle(/Interactive artifact/) as HTMLIFrameElement;
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Close'}));
        if (ready) {
            act(() => {
                window.dispatchEvent(new MessageEvent('message', {data: {mmArtifact: 1, type: 'ready', token: TOKEN}, origin: 'null', source: frame.contentWindow}));
            });
        }
        act(() => {
            screen.getByTestId(sentinel).focus();
        });
        expect(document.activeElement).toBe(screen.getByRole('button', {name: expected}));
        advance(1000);
        expect(stopped()).toBeNull();
    });

    test('an artifact that takes focus from the viewer\'s header controls is stopped', async () => {
        render(withProviders(
            <HTMLArtifactFullscreen
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='dash.html'
                onClose={jest.fn()}
            />,
        ));
        await flushToken();
        const frame = screen.getByTitle(/Interactive artifact/) as HTMLIFrameElement;
        const close = screen.getByRole('button', {name: 'Close'});
        expect(document.activeElement).toBe(close);
        act(() => frame.focus());
        advance(300);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
        expect(document.activeElement).toBe(close);
    });

    test('skips controls without a layout box', async () => {
        render(withProviders(
            <HTMLArtifactFullscreen
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName='dash.html'
                onClose={jest.fn()}
            />,
        ));
        await flushToken();
        const download = screen.getByRole('button', {name: 'Download'});
        rects.mockImplementation(function rectsFor(this: HTMLElement) {
            return (this === download ? [] : [{}]) as unknown as DOMRectList;
        });
        act(() => {
            screen.getByTestId('html-artifact-fullscreen-sentinel-end').focus();
        });
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Reload'}));
    });
});

describe('artifact safety notice', () => {
    test.each([
        {name: 'inline card', fileName: 'card.html', view: 'card', area: 'html-artifact-frame'},
        {name: 'inline card before Run', fileName: 'card.html', view: 'placeholder', area: 'html-artifact-run-placeholder'},
        {name: 'fullscreen viewer', fileName: 'full.html', view: 'fullscreen', area: 'html-artifact-frame'},
    ])('$name always shows the notice outside the frame area', async ({fileName, view, area}) => {
        render(withProviders(view === 'fullscreen' ? (
            <HTMLArtifactFullscreen
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName={fileName}
                onClose={jest.fn()}
            />
        ) : (
            <HTMLArtifactCard
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileName={fileName}
                autoRun={view === 'card'}
            />
        )));
        await flushToken();
        const notice = screen.getByRole('note');
        expect(notice.getAttribute('aria-label')).toMatch(/AI-generated.*Never enter passwords/);
        expect(notice.textContent).toContain('Don\'t enter passwords');
        expect(screen.getByTestId(area).contains(notice)).toBe(false);
    });
});

describe('HTMLArtifactCard run gate', () => {
    const FILE_ID = 'abcdefghijklmnopqrstuvwxyz';
    const placeholder = () => screen.queryByTestId('html-artifact-run-placeholder');

    function renderCard(autoRun: boolean, fileId = FILE_ID) {
        return render(withProviders(
            <HTMLArtifactCard
                fileId={fileId}
                fileName='run.html'
                autoRun={autoRun}
            />,
        ));
    }

    test('the requester\'s card loads right away, not engaged', async () => {
        renderCard(true);
        await flushToken();
        expect(placeholder()).toBeNull();
        expect(frameIframe()).not.toBeNull();
        expect(screen.getByTestId('html-artifact-frame').getAttribute('data-engaged')).toBe('false');
        expect(screen.getByTestId('html-artifact-reload')).toBeTruthy();
    });

    test('anyone else sees Run and no frame until a trusted click on Run', async () => {
        renderCard(false);
        await flushToken();
        expect(placeholder()?.textContent).toContain('runs code only when you choose');
        expect(frameIframe()).toBeNull();
        expect(screen.queryByTestId('html-artifact-frame')).toBeNull();
        expect(screen.queryByTestId('html-artifact-reload')).toBeNull();
        expect(mockGetToken).not.toHaveBeenCalled();

        fireEvent.click(screen.getByTestId('html-artifact-run'));
        await flushToken();
        expect(frameIframe()).toBeNull();

        trustedClick(screen.getByTestId('html-artifact-run'));
        await flushToken();
        expect(placeholder()).toBeNull();
        expect(frameIframe()).not.toBeNull();
        expect(screen.getByTestId('html-artifact-frame').getAttribute('data-engaged')).toBe('false');
        expect(screen.getByTestId('html-artifact-reload')).toBeTruthy();
    });

    test('Run loads the artifact covered: clicking Run does not give it keyboard focus', async () => {
        renderCard(false);
        trustedClick(screen.getByTestId('html-artifact-run'));
        await flushToken();
        const frame = frameIframe()!;
        act(() => {
            window.dispatchEvent(new MessageEvent('message', {data: {mmArtifact: 1, type: 'ready', token: TOKEN}, origin: 'null', source: frame.contentWindow}));
        });
        expect(screen.getByTestId('html-artifact-interact')).toBeTruthy();
        act(() => frame.focus());
        advance(300);
        expect(stopped()?.getAttribute('data-stop-reason')).toBe('focus');
    });

    test('Run is remembered per viewer and file for the page session', async () => {
        const first = renderCard(false);
        trustedClick(screen.getByTestId('html-artifact-run'));
        await flushToken();
        first.unmount();

        const again = renderCard(false);
        await flushToken();
        expect(placeholder()).toBeNull();
        expect(frameIframe()).not.toBeNull();

        // Not clicked in this view: it mounts covered.
        expect(screen.getByTestId('html-artifact-frame').getAttribute('data-engaged')).toBe('false');
        again.unmount();

        renderCard(false, 'zyxwvutsrqponmlkjihgfedcba');
        await flushToken();
        expect(placeholder()).not.toBeNull();
    });

    test('runs once the viewer turns out to be the requester', async () => {
        const {rerender} = renderCard(false);
        await flushToken();
        expect(placeholder()).not.toBeNull();
        rerender(withProviders(
            <HTMLArtifactCard
                fileId={FILE_ID}
                fileName='run.html'
                autoRun={true}
            />,
        ));
        await flushToken();
        expect(placeholder()).toBeNull();
        expect(frameIframe()).not.toBeNull();
    });

    test('opening fullscreen from the placeholder runs the artifact there, covered', async () => {
        renderCard(false);
        trustedClick(screen.getByTestId('html-artifact-open-fullscreen'));
        await flushToken();
        const viewer = screen.getByTestId('html-artifact-fullscreen');
        const frames = screen.getAllByTestId('html-artifact-frame');
        const fullscreenFrame = frames.find((f) => viewer.contains(f))!;
        expect(fullscreenFrame.getAttribute('data-engaged')).toBe('false');
        expect(fullscreenFrame.querySelector('iframe')).not.toBeNull();
        expect(placeholder()).toBeNull();
    });
});
