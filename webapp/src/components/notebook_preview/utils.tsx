export const joinText = (text: string|string[]): string => {
    if (typeof text !== 'string') {
        return text.map(joinText).join('');
    }
    return text;
};
