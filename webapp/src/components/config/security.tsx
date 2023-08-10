import React from 'react';
import styled from 'styled-components';

export type SecurityConfig = {
    enableUserRestrictions: boolean
    allowPrivateChannels: boolean
    allowedTeamIds: string
    onlyUsersOnTeam: string
}

const SecurityContainer = styled.div`
    position: relative;
    padding: 20px;
    border: 1px solid #ccc;
    margin-bottom: 20px;
    background: white;
    .form-group {
        margin: 20px;
    }
`

type Props = {
    securityConfig: SecurityConfig
    onChange: (service: SecurityConfig) => void
}

const Security = ({securityConfig, onChange}: Props) => {
    return (
        <SecurityContainer className='AdminPanel'>
            <div className='header'>
                <h3><span>User restrictions</span></h3>
                <div className='mt-2'><span>Enable restrictions to allow or not users to use AI in this instance.</span></div>
            </div>
            <div className='form-group'>
                <label
                    className='control-label col-sm-4'
                >
                    Enable User Restrictions:
                </label>
                <div className="col-sm-8">
                    <label className="radio-inline">
                        <input
                            type="radio"
                            value="true"
                            checked={securityConfig.enableUserRestrictions}
                            onChange={() => onChange({...securityConfig, enableUserRestrictions: true})}
                        />
                        <span>true</span>
                    </label>
                    <label className="radio-inline">
                        <input
                            type="radio"
                            value="false"
                            checked={!securityConfig.enableUserRestrictions}
                            onChange={() => onChange({...securityConfig, enableUserRestrictions: false})}
                        />
                        <span>false</span>
                    </label>
                    <div className="help-text"><span>Global flag for all below settings.</span></div>
                </div>
            </div>
            {securityConfig.enableUserRestrictions && (
                <>
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                        >
                            Allow Private Channels:
                        </label>
                        <div className="col-sm-8">
                            <label className="radio-inline">
                                <input
                                    type="radio"
                                    value="true"
                                    checked={securityConfig.allowPrivateChannels}
                                    onChange={() => onChange({...securityConfig, allowPrivateChannels: true})}
                                />
                                <span>true</span>
                            </label>
                            <label className="radio-inline">
                                <input
                                    type="radio"
                                    value="false"
                                    checked={!securityConfig.allowPrivateChannels}
                                    onChange={() => onChange({...securityConfig, allowPrivateChannels: false})}
                                />
                                <span>false</span>
                            </label>
                        </div>
                    </div>
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-allow-team-ids'
                        >
                            Allow Team IDs (csv):
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-allow-team-ids'
                                className='form-control'
                                type="text"
                                value={securityConfig.allowedTeamIds}
                                onChange={(e) => onChange({...securityConfig, allowedTeamIds: e.target.value})}
                            />
                        </div>
                    </div>
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-only-users-on-team'
                        >
                            Only Users on Team:
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-only-users-on-team'
                                className='form-control'
                                type="text"
                                value={securityConfig.onlyUsersOnTeam}
                                onChange={(e) => onChange({...securityConfig, onlyUsersOnTeam: e.target.value})}
                            />
                        </div>
                    </div>
                </>
            )}
        </SecurityContainer>
    )
}
export default Security;
