import {WebSocketMessage} from '@mattermost/client';
import React, {ChangeEvent, useEffect, useState} from 'react';
import styled from 'styled-components';

import {generateStatusUpdate} from './client';

import GenericModal from './components/generic_modal';
import {PostUpdateWebsocketMessage} from './components/llmbot_post';
import PostEventListener from './websocket';

const modals = window.WebappUtils.modals;
const Textbox = window.Components.Textbox;

export function makePlaybookRunStatusUpdateHandler(dispatch: any, postEventListener: PostEventListener) {
    return (playbookRunId: string) => {
        dispatch(modals.openModal({
            modalId: Date.now(),
            dialogType: PlaybooksPostUpdateWithAIModal,
            dialogProps: {
                playbookRunId,
                websocketRegister: postEventListener.registerPostUpdateListener,
                websocketUnregister: postEventListener.unregisterPostUpdateListener,
            },
        }));
    };
}

type Props = {
    playbookRunId: string
    websocketRegister: (postID: string, handler: (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => void) => void;
    websocketUnregister: (postID: string) => void;
}

const PlaybooksPostUpdateWithAIModal = (props: Props) => {
    const [update, setUpdate] = useState('');
    const [generating, setGenerating] = useState(false);

    useEffect(() => {
        props.websocketRegister('playbooks_post_update', (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
            const data = msg.data;
            if (!data.control) {
                setGenerating(true);
                setUpdate(data.next);
            } else if (data.control === 'end') {
                setGenerating(false);
            }
        });

        generateStatusUpdate(props.playbookRunId);

        return () => {
            props.websocketUnregister('playbooks_post_update');
        };
    }, []);

    return (
        <SizedGenericModal
            modalHeaderText={'Post update'}
            confirmButtonText={'Post update'}
            cancelButtonText={'Cancel'}
            handleConfirm={() => {
                console.log('do the thing ' + props.playbookRunId);
            }}
            showCancel={true}
            onHide={() => null}
        >
            <Textbox
                tabIndex={0}
                value={update}
                emojiEnabled={true}
                supportsCommands={false}
                suggestionListPosition='bottom'
                useChannelMentions={false}
                onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setUpdate(e.target.value)}
                characterLimit={10000}
                createMessage={''}
                onKeyPress={() => true}
                openWhenEmpty={true}
                channelId={''}
                disabled={false}
            />

        </SizedGenericModal>
    );
};

const SizedGenericModal = styled(GenericModal)`
    width: 768px;
    height: 600px;
    padding: 0;
`;
