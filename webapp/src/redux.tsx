import {combineReducers, Store, Action} from 'redux';
import {GlobalState} from '@mattermost/types/lib/store';

import {callsPostButtonClickedHandler} from './calls_button';

type WebappStore = Store<GlobalState, Action<Record<string, unknown>>>

const CallsClickHandler = 'calls_post_button_clicked_handler';

export async function setupRedux(registry: any, store: WebappStore) {
    const reducer = combineReducers({
        callsPostButtonClicked,
    });

    registry.registerReducer(reducer);
    store.dispatch({
        type: CallsClickHandler as any,
        handler: callsPostButtonClickedHandler,
    });
}

function callsPostButtonClicked(state = false, action: any) {
    switch (action.type) {
    case CallsClickHandler:
        return action.handler || false;
    default:
        return state;
    }
}
