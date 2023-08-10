import React from 'react';
import styled from 'styled-components';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Ansi from 'ansi-to-react';
import DOMPurify from 'dompurify';
import katex from 'katex';

import {joinText} from './utils';

import 'katex/dist/katex.min.css';

type Props = {
    outputType: string
    text: string
    className: string
    traceback: string[]
    cellNumber: number
    data: {[key: string]: string}
}

const CodeCellOutputContainer = styled.div`
    min-height: 1em;
    width: 100%;
    overflow-x: scroll;
    border-right: 1px dotted #CCC;
    display: flex;
    flex-direction: column;
    align-items: flex-start;

    img {
        max-width: 100%;
    }

    &.with-cell-number:before{
        content: "Out [" attr(data-prompt-number) "]:";
        position: absolute;
        font-family: monospace;
        color: #999;
        left: -7.5em;
        width: 7em;
        text-align: right;
    }

    table {
        border: 1px solid #000;
        border-collapse: collapse;
    }
    th {
        font-weight: bold;
    }

    th, td {
        border: 1px solid #000;
        padding: 0.25em;
        text-align: left;
        vertical-align: middle;
        border-collapse: collapse;
    }

    .text-output, .stdout {
        background: var(--center-channel-bg);
        border: 0px;
    }

    .latex-output {
        align-self: center;
    }

    .stdout, stderr {
        white-space: pre-wrap;
        margin: 0;
        padding: 0.1em 0.5em;
    }

    .pyerr, stderr {
        background-color: #ffdddd;
    }
`;

const display_priority = [
    'latex', 'text/latex', 'png', 'image/png', 'jpeg', 'image/jpeg',
    'svg', 'image/svg+xml', 'text/svg+xml', 'html', 'text/html',
    'text/markdown',
    'javascript', 'application/javascript',
    'text', 'text/plain',
];

const CodeCellOutput = ({outputType, className, text, traceback, cellNumber, data}: Props) => {
    let output = null;
    let showCellNumber = true;
    if (outputType === 'pyout' || outputType === 'execute_result' || outputType === 'display_data') {
        const formats = display_priority.filter((d) => {
            return data[d];
        });
        const format = formats[0];
        const content = data[format];
        if (format === 'text/plain' || format === 'text') {
            output = (
                <pre className='text-output'>
                    <Ansi>{joinText(content)}</Ansi>
                </pre>
            );
        } else if (format === 'text/html' || format === 'html') {
            const sanitizedContent = DOMPurify.sanitize(joinText(content));
            output = (
                <div
                    className='html-output'
                    dangerouslySetInnerHTML={{__html: sanitizedContent}}
                />
            );
        } else if (format === 'text/markdown' || format === 'markdown') {
            output = (<ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(content)}</ReactMarkdown>);
        } else if (format === 'text/svg+xml' || format === 'image/svg+xml' || format === 'svg') {
            const sanitizedContent = DOMPurify.sanitize(joinText(content));
            output = (
                <div
                    className='svg-output'
                    dangerouslySetInnerHTML={{__html: sanitizedContent}}
                />
            );
            showCellNumber = false;
        } else if (format === 'text/latex' || format === 'latex') {
            const katexOptions = {
                throwOnError: false,
                displayMode: true,
                maxSize: 200,
                maxExpand: 100,
                fleqn: true,
            };

            let latex = joinText(content);
            if (latex.startsWith('$$') && latex.endsWith('$$')) {
                latex = latex.slice(2, -2);
            }
            const sanitizedText = DOMPurify.sanitize(katex.renderToString(latex, katexOptions));
            output = (
                <div
                    className='latex-output'
                    dangerouslySetInnerHTML={{__html: sanitizedText}}
                />
            );
        } else if (format === 'image/png' || format === 'png') {
            output = (
                <img
                    className='image-output'
                    src={`data:image/png;base64,${joinText(content).replace(/\n/g, '')}`}
                />
            );
            showCellNumber = false;
        } else if (format === 'image/jpeg' || format === 'jpeg' || format === 'jpg') {
            output = (
                <img
                    className='image-output'
                    src={`data:image/jpeg;base64,${joinText(content).replace(/\n/g, '')}`}
                />
            );
            showCellNumber = false;
        }
    } else if (outputType === 'pyerr' || outputType === 'error') {
        output = (
            <pre className='pyerr'>
                <Ansi>{traceback.join('\n')}</Ansi>
            </pre>
        );
        showCellNumber = false;
    } else if (outputType === 'stream' || outputType === 'error') {
        output = (
            <pre className={className}>
                <Ansi>{text}</Ansi>
            </pre>
        );
        showCellNumber = false;
    }

    return (
        <CodeCellOutputContainer
            className={showCellNumber ? 'with-cell-number' : ''}
            data-prompt-number={cellNumber}
        >
            {output}
        </CodeCellOutputContainer>
    );
};

export default CodeCellOutput;
