// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {doRunSearch} from './client';
import {doSelectPost} from './hooks';

export async function handleAskChannelCommand(
    message: string,
    args: {
        channel_id: string;
        team_id: string;
        root_id: string;
    },
    store: any,
    rhs: { showRHSPlugin: any },
) {
    if (!message.trim()) {
        return {
            error: {
                message: 'Please provide a search query after /ask-channel',
            },
        };
    }

    try {
        const result = await doRunSearch(
            message,
            args.team_id,
            args.channel_id,
        );

        // Get store and dispatch actions to select post and open RHS
        doSelectPost(result.postId, result.channelId, store.dispatch);
        store.dispatch(rhs.showRHSPlugin);

        // Return empty object to prevent default error message
        return {};
    } catch (error) {
        return {
            error: {
                message: 'Failed to process search request',
            },
        };
    }
}
