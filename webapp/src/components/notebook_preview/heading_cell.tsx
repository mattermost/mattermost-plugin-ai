import React from 'react';
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {joinText} from './utils'
import styled from 'styled-components';

type Props ={
    source: string
    level: number
    className?: string
}

const HeadingCell = ({source, level, className}: Props) => {
    const content = <ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(source)}</ReactMarkdown>
    if (level === 1) { return <h1 className={className || ''}>{content}</h1> }
    else if (level === 2) { return <h2 className={className || ''}>{content}</h2> }
    else if (level === 3) { return <h3 className={className || ''}>{content}</h3> }
    else if (level === 4) { return <h4 className={className || ''}>{content}</h4> }
    else if (level === 5) { return <h5 className={className || ''}>{content}</h5> }
    else { return <h6 className={className || ''}>{content}</h6> }
}

const SytledHeadingCell = styled(HeadingCell)`
    position: relative;
    + .cell {
        margin-top: 0.5em;
    }
`


export default SytledHeadingCell;
