import {Client4 as Client4Class, ClientError} from '@mattermost/client';

import {manifest} from './manifest';

const Client4 = new Client4Class();

function postRoute(postid: string): string {
    return `/plugins/${manifest.id}/post/${postid}`;
}

function textRoute(): string {
    return `/plugins/${manifest.id}/text`;
}

function channelRoute(channelid: string): string {
    return `/plugins/${manifest.id}/channel/${channelid}`;
}

export async function doReaction(postid: string) {
    const url = `${postRoute(postid)}/react`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doSummarize(postid: string) {
    const url = `${postRoute(postid)}/summarize`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doFeedback(postid: string, positive: boolean) {
    let url = `${postRoute(postid)}/feedback/`;

    if (positive) {
        url += 'positive';
    } else {
        url += 'negative';
    }

    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doTranscribe(postid: string) {
    const url = `${postRoute(postid)}/transcribe`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doSimplify(message: string) {
    const url = `${textRoute()}/simplify`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({message}),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doChangeTone(tone: string, message: string) {
    const url = `${textRoute()}/change_tone/${tone}`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({message}),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doAskAiChangeText(ask: string, message: string) {
    const url = `${textRoute()}/ask_ai_change_text`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({message, ask}),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function summarizeChannelSince(channelID: string, since: number) {
    const url = `${channelRoute(channelID)}/summarize/since`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({since}),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function summarizeThreadSince(threadId: string, since: number) {
    const url = `${postRoute(threadId)}/summarize/since`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({since}),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}
