import React from 'react';

import styled from 'styled-components';

import CodeCellInput from './code_cell_input';
import CodeCellOutput from './code_cell_output';
import {Output} from './types';
import {joinText} from './utils';

type Props ={
    source: string|string[]
    language: string
    cellNumber: number
    outputs: Output[]
}

const CodeCellContainer = styled.div`
    position: relative;

    + .cell {
        margin-top: 0.5em;
    }

    pre, .hljs {
        width: 100%;
    }
`;

const CodeCell = ({source, language, outputs, cellNumber}: Props) => {
    return (
        <CodeCellContainer className='code-cell'>
            <CodeCellInput
                text={joinText(source)}
                language={language}
                cellNumber={cellNumber}
            />
            {outputs && outputs.map((o, idx) => (
                <CodeCellOutput
                    key={idx}
                    text={joinText(o.text || '')}
                    outputType={o.output_type}
                    className={o.stream || o.name || ''}
                    traceback={o.traceback || []}
                    cellNumber={cellNumber}
                    data={o.data || o as any}
                />
            ))}
        </CodeCellContainer>
    );
};

export default CodeCell;
