import React from 'react';
import {escapeHTML} from './utils'
import styled from 'styled-components';

type Props = {
    text: string
    language: string
    cellNumber: number
}

const CodeCellInputtContainer = styled.div`
    &:before {
        content: "In [" attr(data-prompt-number) "]:";
        position: absolute;
        font-family: monospace;
        color: #999;
        left: -7.5em;
        width: 7em;
        text-align: right;
    }
`

const CodeCellInput = ({text, language, cellNumber}: Props) => {
    if (!text.length) { return <div/>; }

    return (
        <CodeCellInputtContainer className='cell-input' data-prompt-number={cellNumber}>
            <pre>
                <code
                    data-language={language}
                    className={`lang-${language}`}
                >
                    {text}
                </code>
            </pre>
        </CodeCellInputtContainer>
    )
}

export default CodeCellInput;
