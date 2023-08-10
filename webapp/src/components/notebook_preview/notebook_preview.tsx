import React, {useEffect, useState} from 'react';

// import nb from 'notebookjs';

import {FileInfo} from '@mattermost/types/files';
import {Client4} from '@mattermost/client';
import styled from 'styled-components';

import Notebook from './notebook';

type Props = {
    fileInfo: FileInfo,
}

const NotebookPreviewContainer = styled.div`
    background: var(--center-channel-bg);
    text-align: left;
    max-height: 100%;
    padding: 100px;
    overflow: scroll;
    max-width: 1024px;
`;

const NotebookPreview = ({fileInfo}: Props) => {
    const [notebook, setNotebook] = useState<any>(null);

    useEffect(() => {
        const client = new Client4();
        fetch(client.getFileUrl(fileInfo.id, new Date().getTime())).then(async (response: any) => {
            const data = await response.json();
            setNotebook(data);
        });
    }, []);

    return (
        <NotebookPreviewContainer>
            {notebook &&
                <Notebook
                    worksheets={notebook.worksheets || [{cells: notebook.cells}]}
                    metadata={notebook.metadata}
                />}
        </NotebookPreviewContainer>
    );
};

export default NotebookPreview;
