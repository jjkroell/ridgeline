// Search engines' renderers, by user agent. They get the desktop routes (the
// canonical URLs) rather than the phone app, and no auto-opened What's new, whose
// text would otherwise sit on every page they index. Keep in step with
// searchCrawlers in internal/api/crawl.go.
const CRAWLERS =
	/googlebot|google-inspectiontool|googleother|bingbot|bingpreview|applebot|duckduckbot|duckassistbot|yandexbot|qwantbot|slurp|oai-searchbot|claude-searchbot|perplexitybot/i;

export const isSearchCrawler = (): boolean =>
	typeof navigator !== 'undefined' && CRAWLERS.test(navigator.userAgent);
