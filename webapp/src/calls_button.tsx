// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {doSummarizeTranscription} from './client';
import {doSelectPost} from './hooks';

export function makeCallsPostButtonClickedHandler(dispatch: any) {
    return async (post: any) => {
        const result = await doSummarizeTranscription(post.id);
        doSelectPost(result.postid, result.channelid, dispatch);
    };
}
