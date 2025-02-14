import React from 'react';
import styled, {keyframes} from 'styled-components';

const spin = keyframes`
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
`;

const SpinnerContainer = styled.div`
    display: inline-block;
    width: 24px;
    height: 24px;
    margin: 8px;
`;

const SpinnerCircle = styled.div`
    width: 100%;
    height: 100%;
    border: 2px solid rgba(var(--center-channel-color-rgb), 0.16);
    border-top-color: rgba(var(--center-channel-color-rgb), 0.64);
    border-radius: 50%;
    animation: ${spin} 1s linear infinite;
`;

const Spinner: React.FC = () => (
    <SpinnerContainer>
        <SpinnerCircle />
    </SpinnerContainer>
);

export default Spinner;
