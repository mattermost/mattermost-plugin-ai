// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package meetings

import (
	"os/exec"
)

const (
	ffmpegPluginPath = "./plugins/mattermost-ai/dist/ffmpeg"
)

// resolveFFMPEGPath checks for ffmpeg installation and returns the appropriate path
func resolveFFMPEGPath() string {
	_, standardPathErr := exec.LookPath("ffmpeg")
	if standardPathErr != nil {
		_, pluginPathErr := exec.LookPath(ffmpegPluginPath)
		if pluginPathErr != nil {
			return ""
		}
		return ffmpegPluginPath
	}

	return "ffmpeg"
}
