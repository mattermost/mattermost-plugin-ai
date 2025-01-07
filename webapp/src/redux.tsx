import {combineReducers, Store, Action} from 'redux';
import {GlobalState} from '@mattermost/types/store';

import {makeCallsPostButtonClickedHandler} from './calls_button';
import {makePlaybookRunStatusUpdateHandler} from './playbooks_button';
import PostEventListener from './websocket';
import manifest from './manifest';
import {useBotlist} from './bots';
import {DropdownBotSelector} from './components/bot_slector'

type WebappStore = Store<GlobalState, Action<Record<string, unknown>>>

const CallsClickHandler = 'calls_post_button_clicked_handler';
const PlaybooksRunStatusUpdateClickHandler = 'playbooks_run_status_update_click_handler';
const AIBotSelectorComponent = 'ai_bots_selector_component';
const AIBotsLoaderHook = 'ai_bots_loader_hook';
export const BotsHandler = manifest.id + '_bots';

export async function setupRedux(registry: any, store: WebappStore, postEventListener: PostEventListener) {
    const reducer = combineReducers({
        callsPostButtonClickedTranscription,
        aiStatusUpdateClicked,
        bots,
        botSelector,
        botsLoaderHook,
        botChannelId,
        selectedPostId,
    });
    registry.registerReducer(reducer);

    store.dispatch({
        type: CallsClickHandler as any,
        handler: makeCallsPostButtonClickedHandler(store.dispatch),
    });
    store.dispatch({
        type: PlaybooksRunStatusUpdateClickHandler as any,
        handler: makePlaybookRunStatusUpdateHandler(store.dispatch, postEventListener),
    });
    store.dispatch({
        type: AIBotSelectorComponent as any,
        component: DropdownBotSelector,
    });

    store.dispatch({
        type: AIBotsLoaderHook as any,
        hook: useBotlist,
    });

    // This is a workaround for a bug where the RHS was inaccessible to
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

function aiStatusUpdateClicked(state = false, action: any) {
    switch (action.type) {
    case PlaybooksRunStatusUpdateClickHandler:
        return action.handler || false;
    default:
        return state;
    }
}

function botSelector(state = false, action: any) {
    switch (action.type) {
    case AIBotSelectorComponent:
        return action.component;
    default:
        return state;
    }
}

function botsLoaderHook(state = false, action: any) {
    switch (action.type) {
    case AIBotsLoaderHook:
        return action.hook;
    default:
        return state;
    }
}

function bots(state = null, action: any) {
    switch (action.type) {
    case BotsHandler:
        return action.bots;
    default:
        return state;
    }
}

function botChannelId(state = '', action: any) {
    switch (action.type) {
    case 'SET_AI_BOT_CHANNEL':
        return action.botChannelId;
    default:
        return state;
    }
}

function selectedPostId(state = '', action: any) {
    switch (action.type) {
    case 'SELECT_AI_POST':
        return action.postId;
    default:
        return state;
    }
}
