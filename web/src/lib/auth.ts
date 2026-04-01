// Shared auth state — readable in all components via $derived(auth.loggedIn) etc.
// Uses svelte-style exports rather than a writable store so it works with Svelte 5 runes.

export const KEY_STORAGE = 'datacat_key';

export function getStoredKey(): string {
	if (typeof localStorage === 'undefined') return '';
	return localStorage.getItem(KEY_STORAGE) ?? '';
}

export function saveKey(key: string) {
	localStorage.setItem(KEY_STORAGE, key);
}

export function clearKey() {
	localStorage.removeItem(KEY_STORAGE);
}
