import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import styled from 'styled-components';

import {joinText} from './utils';

type Props ={
    source: string|string[]
}

const RawCellContainer = styled.div`
    position: relative;
    white-space: pre-wrap;
    background-color: #f5f2f0;
    font-family: Consolas, Monaco, 'Andale Mono', monospace;
    padding: 1em;
    margin: .5em 0;

    + .cell {
        margin-top: 0.5em;
    }
`;

const RawCell = ({source}: Props) => {
    return (
        <RawCellContainer>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(source)}</ReactMarkdown>
        </RawCellContainer>
    );
};

export default RawCell;
