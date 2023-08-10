import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import styled from 'styled-components';

import {joinText} from './utils';

type Props ={
    source: string
    level: number
    className?: string
}

const HeadingCell = ({source, level, className}: Props) => {
    const content = <ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(source)}</ReactMarkdown>;
    switch (level) {
    case 1:
        return <h1 className={className || ''}>{content}</h1>;
    case 2:
        return <h2 className={className || ''}>{content}</h2>;
    case 3:
        return <h3 className={className || ''}>{content}</h3>;
    case 4:
        return <h4 className={className || ''}>{content}</h4>;
    case 5:
        return <h5 className={className || ''}>{content}</h5>;
    default:
        return <h6 className={className || ''}>{content}</h6>;
    }
};

const SytledHeadingCell = styled(HeadingCell)`
    position: relative;
    + .cell {
        margin-top: 0.5em;
    }
`;

export default SytledHeadingCell;
