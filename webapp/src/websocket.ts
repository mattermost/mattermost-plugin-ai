import {WebSocketMessage} from '@mattermost/client';

import {PostUpdateWebsocketMessage} from './components/llmbot_post';

type WebsocketListener = (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => void
type WebsocketListeners = Map<string, WebsocketListener>

export default class PostEventListener {
    postUpdateWebsocketListeners: WebsocketListeners = new Map<string, WebsocketListener>();

    public registerPostUpdateListener = (postID: string, listener: WebsocketListener) => {
        this.postUpdateWebsocketListeners.set(postID, listener);
    };

    public unregisterPostUpdateListener = (postID: string) => {
        this.postUpdateWebsocketListeners.delete(postID);
    };

    public handlePostUpdateWebsockets = (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
        const postID = msg.data.post_id;
        this.postUpdateWebsocketListeners.get(postID)?.(msg);
    };
}
