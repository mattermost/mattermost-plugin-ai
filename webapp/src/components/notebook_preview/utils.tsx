export const joinText = (text: string|string[]): string => {
    if (typeof text !== 'string') {
        return text.map(joinText).join('');
    }
    return text;
};

export const escapeHTML = (text: string) => {
    return text.replace(/</g, '&lt;').replace(/>/g, '&gt;');
};
