import {doSummarizeTranscription} from './client';
import {doSelectPost} from './hooks';

export function makeCallsPostButtonClickedHandler(dispatch: any) {
    return async (post: any) => {
        const result = await doSummarizeTranscription(post.id);
        doSelectPost(result.postid, result.channelid, dispatch);
    };
}
