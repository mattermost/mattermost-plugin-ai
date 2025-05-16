// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';

const professional = 'professional';
const enterprise = 'enterprise';
const enterpriseAdvanced = 'advanced';

// isValidSkuShortName returns whether the SKU short name is one of the known strings;
// namely: professional, enterprise or enterprise advanced.
const isValidSkuShortName = (license: Record<string, string>) => {
    switch (license?.SkuShortName) {
    case professional:
    case enterprise:
    case enterpriseAdvanced:
        return true;
    default:
        return false;
    }
};

export const checkEnterpriseLicensed = (license: Record<string, string>) => {
    if (license?.SkuShortName === enterprise || license?.SkuShortName === enterpriseAdvanced) {
        return true;
    }

    if (!isValidSkuShortName(license)) {
        // As a fallback for licenses whose SKU short name is unknown, make a best effort to try
        // and use the presence of a known E20/Enterprise feature as a check to determine licensing.
        if (license?.MessageExport === 'true') {
            return true;
        }
    }

    return false;
};

export const checkProfessionalLicensed = (license: Record<string, string>) => {
    if (license?.SkuShortName === professional ||
        license?.SkuShortName === enterprise ||
        license?.SkuShortName === enterpriseAdvanced) {
        return true;
    }

    if (!isValidSkuShortName(license)) {
        // As a fallback for licenses whose SKU short name is unknown, make a best effort to try
        // and use the presence of a known E10/Professional feature as a check to determine licensing.
        if (license?.LDAP === 'true') {
            return true;
        }
    }

    return false;
};

const isConfiguredForDevelopment = (state: GlobalState): boolean => {
    const config = state.entities.general.config;

    return config.EnableTesting === 'true' && config.EnableDeveloper === 'true';
};

// isEnterpriseLicensedOrDevelopment returns true when the server is licensed with minimum Mattermost
// Enterprise License, or has `EnableDeveloper` and `EnableTesting`
// configuration settings enabled, signaling a non-production, developer mode.
export const isEnterpriseLicensedOrDevelopment = (state: GlobalState): boolean => {
    const license = state.entities.general.license;

    return checkEnterpriseLicensed(license) || isConfiguredForDevelopment(state);
};

// isProfressionalLicensedOrDevelopment returns true when the server is at least licensed with a Mattermost Professional License,
// or has `EnableDeveloper` and `EnableTesting` configuration settings enabled,
// signaling a non-production, developer mode.
export const isProfessionalLicensedOrDevelopment = (state: GlobalState): boolean => {
    const license = state.entities.general.license;

    return checkProfessionalLicensed(license) || isConfiguredForDevelopment(state);
};

export function useIsMultiLLMLicensed() {
    return useSelector(isEnterpriseLicensedOrDevelopment);
}

export function useIsBasicsLicensed() {
    return useSelector(isEnterpriseLicensedOrDevelopment);
}
