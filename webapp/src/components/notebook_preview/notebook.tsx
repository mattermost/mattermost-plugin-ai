import React, {useEffect} from 'react';

import styled from 'styled-components';

import hljs from 'highlight.js';

import {WorksheetData, Metadata} from './types';
import Worksheet from './worksheet';
import 'highlight.js/styles/github.css';

type Props = {
    worksheets: WorksheetData[]
    metadata: Metadata
}

const NotebookContainer = styled.div`
    line-height: 1.5;

    blockquote {
        border-left: 5px solid #CCC;
        margin-left: 0;
        padding-left: 1em;
    }

    &&&& pre, &&&& code {
        min-width: unset;
        min-height: unset;
    }

    &&&& a {
        color: var(--link-color);
    }
`;

const Notebook = ({worksheets, metadata}: Props) => {
    const language = metadata.language || (metadata.kernelspec && metadata.kernelspec.language) || (metadata.language_info && metadata.language_info.name);
    useEffect(() => {
        hljs.configure({cssSelector: '.code-cell .cell-input pre code'});
        hljs.highlightAll();
    }, []);

    return (
        <NotebookContainer className='notebook'>
            {worksheets.map((w, idx) => (
                <Worksheet
                    key={idx}
                    cells={w.cells}
                    language={language}
                />
            ))}
        </NotebookContainer>
    );
};

export default Notebook;
