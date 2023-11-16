import {doTranscribe} from './client';

export function callsPostButtonClickedHandler(post: any) {
    doTranscribe(post.id);
}
