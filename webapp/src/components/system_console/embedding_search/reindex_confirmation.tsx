// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import ConfirmationDialog from '../../confirmation_dialog';

interface ReindexConfirmationProps {
    show: boolean;
    onConfirm: () => void;
    onCancel: () => void;
}

export const ReindexConfirmation = ({show, onConfirm, onCancel}: ReindexConfirmationProps) => {
    if (!show) {
        return null;
    }

    return (
        <ConfirmationDialog
            title={<FormattedMessage defaultMessage='Confirm Reindexing'/>}
            message={
                <>
                    <p>
                        <FormattedMessage defaultMessage='Are you sure you want to reindex all posts?'/>
                    </p>
                    <p>
                        <FormattedMessage defaultMessage='This will clear the current index and rebuild it from scratch. The process will:'/>
                    </p>
                    <ul>
                        <li><FormattedMessage defaultMessage='Index all existing posts in the database'/></li>
                        <li><FormattedMessage defaultMessage='Take a significant amount of time for large installations'/></li>
                        <li><FormattedMessage defaultMessage='Increase database load during the reindexing process'/></li>
                    </ul>
                </>
            }
            confirmButtonText={<FormattedMessage defaultMessage='Reindex'/>}
            onConfirm={onConfirm}
            onCancel={onCancel}
        />
    );
};