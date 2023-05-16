import {ClientError} from '@mattermost/client';
import {Client4} from 'mattermost-redux/client';

export async function doReaction(postid: string) {
    const url = '/plugins/summarize/react/' + postid;
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
    let url = '/plugins/summarize/feedback/post/' + postid + '/';

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

