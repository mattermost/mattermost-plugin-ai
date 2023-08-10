import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Ansi from 'ansi-to-react';
import DOMPurify from 'dompurify';
import katex from 'katex';

import {joinText} from './utils';

import 'katex/dist/katex.min.css';

type Props = {
    format: string
    content: string|string[]
}

const CodeCellOutputData = ({format, content}: Props) => {
    const sanitizedContent = DOMPurify.sanitize(joinText(content));
    switch (format) {
        case 'text/plain':
        case 'text':
            return (
                <pre className='text-output'>
                    <Ansi>{sanitizedContent}</Ansi>
                </pre>
            );
        case 'text/html':
        case 'html':
            return (
                <div
                    className='html-output'
                    dangerouslySetInnerHTML={{__html: sanitizedContent}}
                />
            );
        case 'text/markdown':
        case 'markdown':
            return (<ReactMarkdown remarkPlugins={[remarkGfm]}>{joinText(content)}</ReactMarkdown>);
        case 'text/svg+xml':
        case 'image/svg+xml':
        case 'svg':
            return (
                <div
                    className='svg-output'
                    dangerouslySetInnerHTML={{__html: sanitizedContent}}
                />
            );
        case 'text/latex':
        case 'latex':
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
            return (
                <div
                    className='latex-output'
                    dangerouslySetInnerHTML={{__html: sanitizedText}}
                />
            );
        case 'image/png':
        case 'png':
            return (
                <img
                    className='image-output'
                    src={`data:image/png;base64,${joinText(content).replace(/\n/g, '')}`}
                />
            );
        case 'image/jpeg':
        case 'jpeg':
        case 'jpg':
            return (
                <img
                    className='image-output'
                    src={`data:image/jpeg;base64,${joinText(content).replace(/\n/g, '')}`}
                />
            );
    }
    return null
};

export default CodeCellOutputData;
