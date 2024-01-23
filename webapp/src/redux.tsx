import {combineReducers, Store, Action} from 'redux';
import {GlobalState} from '@mattermost/types/lib/store';

import {makeCallsPostButtonClickedHandler} from './calls_button';

type WebappStore = Store<GlobalState, Action<Record<string, unknown>>>

const CallsClickHandler = 'calls_post_button_clicked_handler';

export async function setupRedux(registry: any, store: WebappStore) {
    const reducer = combineReducers({
        callsPostButtonClickedTranscription,
    });

    registry.registerReducer(reducer);
    store.dispatch({
        type: CallsClickHandler as any,
        handler: makeCallsPostButtonClickedHandler(store.dispatch),
    });

    // This is a workaround for a bug where the the RHS was inaccessable to
    // users that where not system admins. This is unable to be fixed properly
    // because the Webapp does not export the AdvancedCreateComment directly.
    // #120 filed to remove this workaround.
    store.dispatch({
        type: 'RECEIVED_MY_CHANNEL_MEMBER' as any,
        data: {
            channel_id: undefined, // eslint-disable-line no-undefined
            roles: 'special_workaround',
        },
    });
    store.dispatch({
        type: 'RECEIVED_ROLE' as any,
        data: {
            name: 'special_workaround',
            permissions: ['create_post'],
        },
    });
}

function callsPostButtonClickedTranscription(state = false, action: any) {
    switch (action.type) {
    case CallsClickHandler:
        return action.handler || false;
    default:
        return state;
    }
}
