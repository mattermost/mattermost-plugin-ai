import { Page } from 'playwright';

export const login = async (page: Page, url: string, username: string, password: string) => {
    await page.addInitScript(() => { localStorage.setItem('__landingPageSeen__', 'true'); });
    await page.goto(url);
    await page.getByText('Log in to your account').waitFor();
    await page.getByPlaceholder('Password').fill(password);
    await page.getByPlaceholder("Email or Username").fill(username);
    await page.getByTestId('saveSetting').click();
}
