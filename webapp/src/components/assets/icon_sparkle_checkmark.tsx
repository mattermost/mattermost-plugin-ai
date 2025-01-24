// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import Svg from '../svg';

const IconSparkleCheckmark = (props: {className?: string}) => (
    <Svg
        className={props.className}
        width='16'
        height='16'
        viewBox='0 0 16 16'
        fill='none'
        xmlns='http://www.w3.org/2000/svg'
    >
        <path
            d='M15.1999 4.00634L5.59993 13.6063L1.20312 9.19034L2.33592 8.07674L5.59993 11.3407L14.0671 2.87354L15.1999 4.00634Z'
            fill='currentColor'
        />
        <path
            d='M5.6001 0L4.7201 3.12L1.6001 4L4.7201 4.88L5.6001 8L6.4801 4.88L9.6001 4L6.4801 3.12L5.6001 0Z'
            fill='currentColor'
        />
        <path
            d='M12.7999 9.6001L12.2719 11.4721L10.3999 12.0001L12.2719 12.5281L12.7999 14.4001L13.3279 12.5281L15.1999 12.0001L13.3279 11.4721L12.7999 9.6001Z'
            fill='currentColor'
        />
    </Svg>
);

export default IconSparkleCheckmark;
