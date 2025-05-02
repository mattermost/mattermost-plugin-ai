// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {WebSocketMessage} from '@mattermost/client';

import {PostUpdateWebsocketMessage} from './components/llmbot_post';

import {PostEditedWebsocketEvent} from './index';

type WebsocketListener = (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => void
type WebsocketListenerObject = {
    postID: string;
    listenerID: string;
    listener: WebsocketListener;
}
type WebsocketListeners = WebsocketListenerObject[]

export default class PostEventListener {
    postUpdateWebsocketListeners: WebsocketListeners = [];

    public registerPostUpdateListener = (postID: string, listenerID: string, listener: WebsocketListener) => {
        this.postUpdateWebsocketListeners.push({postID, listenerID, listener});
    };

    public unregisterPostUpdateListener = (postID: string, listenerID: string) => {
        this.postUpdateWebsocketListeners = this.postUpdateWebsocketListeners.filter((listenerObject) => {
            const isSamePostID = listenerObject.postID === postID;
            const isSameListenerID = listenerObject.listenerID === listenerID;
            return !(isSamePostID && isSameListenerID);
        });
    };

    public handlePostUpdateWebsockets = (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
        let postID: string;
        if (msg.event === PostEditedWebsocketEvent) {
            try {
                const post = JSON.parse(msg.data.post ?? '{}');
                postID = post.id;
                msg.data = post;
            } catch (e) {
                // ignore malformed post_edited message
                return;
            }
        } else {
            postID = msg.data.post_id;
        }
        this.postUpdateWebsocketListeners.forEach((listenerObject) => {
            if (listenerObject.postID === postID) {
                listenerObject.listener(msg);
            }
        });
    };
}
