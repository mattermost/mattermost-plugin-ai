// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import Svg from '../svg';

const IconTool = (props: {className?: string}) => (
    <Svg
        className={props.className}
        width='16'
        height='16'
        viewBox='0 0 16 16'
        fill='none'
        xmlns='http://www.w3.org/2000/svg'
    >
        <path
            d='M13.5 2C14.3284 2 15 2.67157 15 3.5V12.5C15 13.3284 14.3284 14 13.5 14H2.5C1.67157 14 1 13.3284 1 12.5V3.5C1 2.67157 1.67157 2 2.5 2H13.5ZM13.5 3.5H2.5V12.5H13.5V3.5ZM6.5 5C7.32843 5 8 5.67157 8 6.5V8.5C8 9.32843 7.32843 10 6.5 10H4.5C3.67157 10 3 9.32843 3 8.5V6.5C3 5.67157 3.67157 5 4.5 5H6.5ZM11.5 5C11.7761 5 12 5.22386 12 5.5C12 5.77614 11.7761 6 11.5 6H9.5C9.22386 6 9 5.77614 9 5.5C9 5.22386 9.22386 5 9.5 5H11.5ZM11.5 7C11.7761 7 12 7.22386 12 7.5C12 7.77614 11.7761 8 11.5 8H9.5C9.22386 8 9 7.77614 9 7.5C9 7.22386 9.22386 7 9.5 7H11.5ZM11.5 9C11.7761 9 12 9.22386 12 9.5C12 9.77614 11.7761 10 11.5 10H9.5C9.22386 10 9 9.77614 9 9.5C9 9.22386 9.22386 9 9.5 9H11.5Z'
            fill='currentColor'
        />
    </Svg>
);

export default IconTool;