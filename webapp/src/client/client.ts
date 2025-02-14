import {Client4} from 'mattermost-redux/client';
import {ClientError} from 'mattermost-redux/client/client4';

export default class Client {
    url: string;

    constructor() {
        this.url = '/plugins/ai/api/v1';
    }

    executeActions = async (actions: any[]) => {
        return this.doPost(
            `${this.url}/actions/execute`,
            {actions},
        );
    };

    doPost = async (url: string, body: any, headers: {[x: string]: string} = {}) => {
        const options = {
            method: 'POST',
            body: JSON.stringify(body),
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
                ...headers,
            },
        };

        const response = await fetch(url, Client4.getOptions(options));

        if (response.ok) {
            return response.json();
        }

        const data = await response.json();
        throw new ClientError(Client4.url, {
            message: data.message || '',
            status_code: response.status,
            url,
        });
    };
}

const client = new Client();
export {client};
