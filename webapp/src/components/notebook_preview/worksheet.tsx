import React from 'react';

import {Cell} from './types';
import MarkdownCell from './markdown_cell';
import HeadingCell from './heading_cell';
import RawCell from './raw_cell';
import CodeCell from './code_cell';

type Props = {
    cells: Cell[]
    language: string
}

const Worksheet = ({cells, language}: Props) => {
    return (
        <div className='worksheet'>
            {cells.map((c, idx) => {
                if (c.cell_type === 'markdown') {
                    return (
                        <MarkdownCell
                            key={idx}
                            source={c.source}
                        />
                    );
                } else if (c.cell_type === 'heading') {
                    return (
                        <HeadingCell
                            key={idx}
                            source={c.source}
                            level={c.level || 1}
                        />
                    );
                } else if (c.cell_type === 'raw') {
                    return (
                        <RawCell
                            key={idx}
                            source={c.source}
                        />
                    );
                } else if (c.cell_type === 'code') {
                    return (
                        <CodeCell
                            key={idx}
                            source={c.input || [c.source]}
                            outputs={c.outputs || []}
                            language={c.language || language}
                            cellNumber={c.prompt_number && c.prompt_number > -1 ? c.prompt_number : c.execution_count}
                        />
                    );
                }
                return null;
            })}
        </div>
    );
};

export default Worksheet;
