// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';

import ArtifactConsentPrompt, {CONSENT_ARM_DELAY_MS} from './artifact_consent_prompt';

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

beforeEach(() => {
    jest.useFakeTimers();
});

afterEach(() => {
    jest.useRealTimers();
});

const arm = () => act(() => {
    jest.advanceTimersByTime(CONSENT_ARM_DELAY_MS);
});

function renderPrompt() {
    const onAllow = jest.fn();
    const onDeny = jest.fn();
    render(
        <ArtifactConsentPrompt
            fileName='dash.html'
            onAllow={onAllow}
            onDeny={onDeny}
        />,
    );
    return {
        onAllow,
        onDeny,
        allow: screen.getByTestId('html-artifact-consent-allow'),
        deny: screen.getByTestId('html-artifact-consent-deny'),
    };
}

describe('ArtifactConsentPrompt pointer-entry gate', () => {
    for (const button of ['allow', 'deny'] as const) {
        const handlerOf = (p: ReturnType<typeof renderPrompt>) => (button === 'allow' ? p.onAllow : p.onDeny);

        const cases: Array<{name: string; enterBeforeArm: boolean; armed: boolean; enterAfterArm: boolean; press: boolean; detail: number; expectCalled: boolean}> = [
            {name: 'pointer click with no pointerenter is ignored', enterBeforeArm: false, armed: true, enterAfterArm: false, press: true, detail: 1, expectCalled: false},
            {name: 'pointer click after a pointerenter that preceded arming is ignored', enterBeforeArm: true, armed: true, enterAfterArm: false, press: true, detail: 1, expectCalled: false},
            {name: 'pointer click after a post-arm pointerenter counts', enterBeforeArm: false, armed: true, enterAfterArm: true, press: true, detail: 1, expectCalled: true},
            {name: 'post-arm pointerenter without a press on the button is ignored', enterBeforeArm: false, armed: true, enterAfterArm: true, press: false, detail: 1, expectCalled: false},
            {name: 'keyboard activation needs no pointerenter', enterBeforeArm: false, armed: true, enterAfterArm: false, press: false, detail: 0, expectCalled: true},
            {name: 'early pointer click is blocked by the arm delay', enterBeforeArm: true, armed: false, enterAfterArm: false, press: true, detail: 1, expectCalled: false},
            {name: 'early keyboard activation is blocked by the arm delay', enterBeforeArm: false, armed: false, enterAfterArm: false, press: false, detail: 0, expectCalled: false},
        ];

        test.each(cases)(`${button}: $name`, (c) => {
            const prompt = renderPrompt();
            const el = prompt[button];
            if (c.enterBeforeArm) {
                fireEvent.pointerEnter(el);
            }
            if (c.armed) {
                arm();
            }
            if (c.enterAfterArm) {
                fireEvent.pointerEnter(el);
            }
            if (c.press) {
                fireEvent.pointerDown(el);
            }
            fireEvent.click(el, {detail: c.detail});
            expect(handlerOf(prompt)).toHaveBeenCalledTimes(c.expectCalled ? 1 : 0);
        });
    }

    test('entering Deny does not unlock Allow', () => {
        const {allow, deny, onAllow} = renderPrompt();
        arm();
        fireEvent.pointerEnter(deny);
        fireEvent.pointerDown(allow);
        fireEvent.click(allow, {detail: 1});
        expect(onAllow).not.toHaveBeenCalled();
    });

    test.each(['allow', 'deny'] as const)('%s: a press that began before arming does not count after leaving and re-entering', (button) => {
        const prompt = renderPrompt();
        const el = prompt[button];
        fireEvent.pointerEnter(el);
        fireEvent.pointerDown(el);
        arm();
        fireEvent.pointerLeave(el);
        fireEvent.pointerEnter(el);
        fireEvent.click(el, {detail: 1});
        expect(prompt.onAllow).not.toHaveBeenCalled();
        expect(prompt.onDeny).not.toHaveBeenCalled();
    });

    test('Enter on the default-focused Deny works once armed', () => {
        const {deny, onDeny} = renderPrompt();
        expect(document.activeElement).toBe(deny);
        arm();
        fireEvent.click(deny, {detail: 0});
        expect(onDeny).toHaveBeenCalledTimes(1);
    });
});
