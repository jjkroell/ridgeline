// The emergency-channel survey (api/survey.go): its id and the choices, shared by the
// form and the admin tallies. The ids must match the server's lists.
export const SURVEY_ID = 'mayday-2026';
export const SURVEY_DONE_KEY = 'ridgeline-survey-mayday-2026';

export const USED = [
	{ id: 'posted', label: 'I posted on #sos' },
	{ id: 'radio', label: 'I had #sos on my radio or app' },
	{ id: 'listed', label: 'I was on the monitoring list' },
	{ id: 'emails', label: 'I got the alert emails' },
	{ id: 'alarm', label: 'I used the sound or notification alarm' },
	{ id: 'read', label: 'I read #sos on Ridgeline' },
	{ id: 'never', label: 'I never used it' }
];
export const MONITOR = [
	{ id: 'yes', label: 'Yes' },
	{ id: 'no', label: 'No' },
	{ id: 'unsure', label: 'Not sure' }
];
export const FEATURES = [
	{ id: 'emails', label: 'Alert emails' },
	{ id: 'list', label: 'A list of who is monitoring' },
	{ id: 'rings', label: 'A ring on the caller’s node on the maps' },
	{ id: 'bot', label: 'KOD-BOT’s auto reply' },
	{ id: 'alarm', label: 'A sound or notification in the browser' },
	{ id: 'guide', label: 'A guide to the channel' }
];
