// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package i18n

// FormatAnalysisPostMessage formats a message for thread analysis posts based on the analysis type
func FormatAnalysisPostMessage(bundle *Bundle, locale string, postIDToAnalyze string, analysisType string, siteURL string) string {
	T := LocalizerFunc(bundle, locale)
	switch analysisType {
	case "summarize_thread":
		return T("copilot.summarize_thread", "Sure, I will summarize this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	case "action_items":
		return T("copilot.find_action_items", "Sure, I will find action items in this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	case "open_questions":
		return T("copilot.find_open_questions", "Sure, I will find open questions in this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	default:
		return T("copilot.analyze_thread", "Sure, I will analyze this thread: %s/_redirect/pl/%s\n", siteURL, postIDToAnalyze)
	}
}
