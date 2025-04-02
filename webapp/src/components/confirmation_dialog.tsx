// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';

import {PrimaryButton, TertiaryButton, DestructiveButton} from './assets/buttons';

interface ConfirmationDialogProps {
    title: React.ReactNode;
    message: React.ReactNode;
    confirmButtonText: React.ReactNode;
    cancelButtonText?: React.ReactNode;
    onConfirm: () => void;
    onCancel: () => void;
    isDestructive?: boolean;
}

const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({
    title,
    message,
    confirmButtonText,
    cancelButtonText = <FormattedMessage defaultMessage='Cancel'/>,
    onConfirm,
    onCancel,
    isDestructive = false,
}) => {
    return (
        <DialogWrapper onClick={onCancel}>
            <DialogContent onClick={(e) => e.stopPropagation()}>
                <DialogHeader>
                    <DialogTitle>{title}</DialogTitle>
                </DialogHeader>
                <DialogBody>
                    {message}
                </DialogBody>
                <DialogFooter>
                    <TertiaryButton onClick={onCancel}>
                        {cancelButtonText}
                    </TertiaryButton>
                    {isDestructive ? (
                        <DestructiveButton onClick={onConfirm}>
                            {confirmButtonText}
                        </DestructiveButton>
                    ) : (
                        <PrimaryButton onClick={onConfirm}>
                            {confirmButtonText}
                        </PrimaryButton>
                    )}
                </DialogFooter>
            </DialogContent>
        </DialogWrapper>
    );
};

const DialogWrapper = styled.div`
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
`;

const DialogContent = styled.div`
    background-color: var(--center-channel-bg);
    border-radius: 8px;
    width: 100%;
    max-width: 512px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
`;

const DialogHeader = styled.div`
    padding: 24px 24px 0;
`;

const DialogTitle = styled.h2`
    font-size: 22px;
    font-weight: 600;
    margin: 0;
    color: var(--center-channel-color);
`;

const DialogBody = styled.div`
    padding: 24px;
    color: rgba(var(--center-channel-color-rgb), 0.72);
    font-size: 14px;
    line-height: 20px;
`;

const DialogFooter = styled.div`
    padding: 0 24px 24px;
    display: flex;
    justify-content: flex-end;
    gap: 12px;
`;

export default ConfirmationDialog;