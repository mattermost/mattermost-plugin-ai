// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import Svg from '../svg';

const IconCheckCircle = (props: {className?: string}) => (
    <Svg
        className={props.className}
        width='16'
        height='16'
        viewBox='0 0 16 16'
        fill='none'
        xmlns='http://www.w3.org/2000/svg'
    >
        <path
            d='M8 1C11.866 1 15 4.13401 15 8C15 11.866 11.866 15 8 15C4.13401 15 1 11.866 1 8C1 4.13401 4.13401 1 8 1ZM8 2.5C4.96243 2.5 2.5 4.96243 2.5 8C2.5 11.0376 4.96243 13.5 8 13.5C11.0376 13.5 13.5 11.0376 13.5 8C13.5 4.96243 11.0376 2.5 8 2.5ZM11.1464 5.14645C11.3417 5.34171 11.3417 5.65829 11.1464 5.85355L7.14645 9.85355C6.95118 10.0488 6.63461 10.0488 6.43934 9.85355L4.85355 8.26776C4.65829 8.0725 4.65829 7.75592 4.85355 7.56066C5.04882 7.36539 5.36539 7.36539 5.56066 7.56066L6.79289 8.79289L10.4393 5.14645C10.6346 4.95118 10.9511 4.95118 11.1464 5.14645Z'
            fill='currentColor'
        />
    </Svg>
);

export default IconCheckCircle;