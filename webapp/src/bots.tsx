import {useState, useEffect} from 'react';

import {useDispatch, useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';

import {getAIBots} from '@/client';

import manifest from './manifest';
import {BotsHandler} from './redux';

export interface LLMBot {
    id: string;
    displayName: string;
    username: string;
    lastIconUpdate: number;
    dmChannelID: string;
}

const defaultBotLocalStorageKey = 'defaultBot';

export const useBotlist = () => {
    const bots = useSelector<GlobalState, LLMBot[] | null>((state: any) => state['plugins-' + manifest.id].bots);
    const [activeBot, setActiveBot] = useState<LLMBot | null>(null);
    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);
    const dispatch = useDispatch();

    // Load bots
    useEffect(() => {
        const fetchBots = async () => {
            const fetchedBots = await getAIBots();
            if (!fetchedBots) {
                return;
            }

            dispatch({
                type: BotsHandler,
                bots: fetchedBots,
            });
        };
        if (!bots) {
            fetchBots();
        }
    }, [currentUserId, bots, dispatch]);

    useEffect(() => {
        const defaultActiveBotName = localStorage.getItem(defaultBotLocalStorageKey);
        setActiveBot(bots?.find((bot: LLMBot) => bot.username === defaultActiveBotName) || bots?.[0] || null);
    }, [bots]);

    useEffect(() => {
        if (!activeBot) {
            return;
        }
        localStorage.setItem(defaultBotLocalStorageKey, activeBot.username);
    }, [activeBot]);

    return {bots, activeBot, setActiveBot};
};
