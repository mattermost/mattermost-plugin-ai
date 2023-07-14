import React, {useState} from 'react';
import IconAI from './icon_ai';

export default function AskAiInput() {
    const [value, setValue] = useState('');
    return (
        <div
            onClick={(e) => e.stopPropagation()}
            style={{
                border: '1px solid #ddd',
                borderRadius: 5,
                display: 'flex',
                padding: 5,
                position: 'relative',
                right: 7,
            }}
        >
            <span className='icon'><IconAI/></span>
            <input
                type='text'
                placeholder='Ask AI about this thread...'
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={(e) => {
                    const postId = ((e.target as HTMLElement).closest('.post-menu') as HTMLElement).dataset.postid
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        console.log('TODO: send message to AI for post', postId, value);
                    }
                }}
                style={{border: 'none', backgroundColor: 'transparent'}}
            />
        </div>
    )
}

