// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import styled, {css, keyframes} from 'styled-components';

import {Icon} from '@mattermost/compass-ui/components/icon';
import {HandRightOutlineIcon} from '@mattermost/compass-icons/components';

// How long the hint stays prominent after the viewer tries to scroll the artifact.
export const NUDGE_MS = 1500;

interface Props {
    onEngage: (e: React.MouseEvent) => void;
}

// Covers an artifact the viewer has not engaged and takes every click on it.
// Scrolling over it scrolls the page, not the artifact, which viewers do not
// expect: trying to makes the hint stand out for a moment. The page's own
// scrolling is left alone.
const ArtifactInteractOverlay = ({onEngage}: Props) => {
    const intl = useIntl();
    const [nudging, setNudging] = useState(false);
    const timerRef = useRef<number | null>(null);

    const nudge = useCallback(() => {
        if (timerRef.current !== null) {
            window.clearTimeout(timerRef.current);
        }
        setNudging(true);
        timerRef.current = window.setTimeout(() => {
            timerRef.current = null;
            setNudging(false);
        }, NUDGE_MS);
    }, []);

    useEffect(() => () => {
        if (timerRef.current !== null) {
            window.clearTimeout(timerRef.current);
        }
    }, []);

    return (
        <Overlay
            type='button'
            aria-label={intl.formatMessage({defaultMessage: 'Interact with this artifact'})}
            onClick={onEngage}
            onWheel={nudge}
            onTouchMove={nudge}
            data-testid='html-artifact-interact'
            data-nudge={nudging}
        >
            <Hint
                aria-hidden='true'
                $nudging={nudging}
                data-testid='html-artifact-interact-hint'
            >
                <Icon
                    size='16'
                    glyph={<HandRightOutlineIcon/>}
                />
                {nudging ? (
                    <FormattedMessage defaultMessage='Click to scroll and interact'/>
                ) : (
                    <FormattedMessage defaultMessage='Click to interact'/>
                )}
            </Hint>
        </Overlay>
    );
};

export default ArtifactInteractOverlay;

// Transparent: the artifact stays fully visible (and readable by assistive
// technology) underneath. The hint keeps to the bottom corner, and stays in
// view while the rest of a tall artifact is scrolled off.
const Overlay = styled.button`
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    justify-content: flex-end;
    width: 100%;
    margin: 0;
    padding: 0;
    border: none;
    background: transparent;
    cursor: pointer;

    &:focus {
        outline: none;
    }

    &:focus-visible {
        box-shadow: inset 0 0 0 2px var(--button-bg);
    }
`;

const pulse = keyframes`
    0% { transform: scale(1); }
    35% { transform: scale(1.12); }
    70% { transform: scale(0.98); }
    100% { transform: scale(1); }
`;

// Opaque in the theme's own colors, so it reads over any artifact content in
// light and dark themes alike.
const Hint = styled.span<{$nudging: boolean}>`
    position: sticky;
    bottom: 8px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin: 8px;
    padding: 4px 10px 4px 6px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
    border-radius: 14px;
    background: var(--center-channel-bg);
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.12);
    color: rgba(var(--center-channel-color-rgb), 0.8);
    font-size: 12px;
    font-weight: 600;
    line-height: 16px;
    white-space: nowrap;
    pointer-events: none;
    transform-origin: right bottom;
    transition: color 0.15s ease-out, background-color 0.15s ease-out, border-color 0.15s ease-out;

    > :first-child {
        color: var(--button-bg);
    }

    ${(props) => (props.$nudging ? css`
        border-color: var(--button-bg);
        background: var(--button-bg);
        box-shadow: 0 6px 14px rgba(0, 0, 0, 0.24);
        color: var(--button-color);
        animation: ${pulse} 0.45s ease-out 2;

        > :first-child {
            color: var(--button-color);
        }
    ` : css`
        ${Overlay}:hover &, ${Overlay}:focus-visible & {
            border-color: rgba(var(--button-bg-rgb), 0.48);
            color: var(--center-channel-color);
        }
    `)}

    @media (prefers-reduced-motion: reduce) {
        animation: none;
        transition: none;
    }
`;
