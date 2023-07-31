import React, {useState} from 'react';
import styled from 'styled-components';

import IconAI from './assets/icon_ai';

const Container = styled.div`
	display: flex
	position: relative
	border: 1px solid #ddd
	borderRadius: 5px
	padding: 5px
	right: 7px
`;

const Input = styled.input`
	border: none
	backgroundColor: transparent
`;

export default function AskAiInput() {
    const [value, setValue] = useState('');
    return (
        <Container onClick={(e) => e.stopPropagation()}>
            <span className='icon'><IconAI/></span>
            <Input
                type='text'
                placeholder='Ask AI about this thread...'
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={(e) => {
                    const postId = ((e.target as HTMLElement).closest('.post-menu') as HTMLElement).dataset.postid;
                    if (e.key === 'Enter') {
                        e.preventDefault();

                        //console.log('TODO: send message to AI for post', postId, value);
                    }
                }}
            />
        </Container>
    );
}

