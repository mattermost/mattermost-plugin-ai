import React from 'react';
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {joinText} from './utils'
import styled from 'styled-components';

type Props ={
    source: string
}

const MarkdownCellContainer = styled.div`
    position: relative;
    + .cell {
        margin-top: 0.5em;
    }

    ul {
        margin-bottom: 10px;
    }
`

const MarkdownCell = ({source}: Props) => {
    return (
        <MarkdownCellContainer>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(source)}</ReactMarkdown>
        </MarkdownCellContainer>
    )
}

export default MarkdownCell;
