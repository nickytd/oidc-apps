// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package parameters

import "strings"

const (
	defaultWatchedDir string = "/etc/kube-rbac-proxy"
	watchedDirParam   string = "--watched-dir="

	defaultCmdLine string = "/usr/local/bin/kube-rbac-proxy"
	cmdLineParam   string = "--cmd-line="

	logLevelParam  string = "--log-level="
	logFormatParam string = "--log-format="
)

// Parameters holds the child process argument and the watched directory.
type Parameters struct {
	CmdLine     string
	CmdLineArgs []string
	WatchedDir  string
	LogLevel    string
	LogFormat   string
}

// Parse returns the parameters based on the supplied arguments.
func Parse(params []string) Parameters {
	parameters := Parameters{
		CmdLine:     defaultCmdLine,
		CmdLineArgs: []string{},
		WatchedDir:  defaultWatchedDir,
		LogLevel:    "info",
		LogFormat:   "json",
	}

	// Extract log-level and log-format before other parsing
	var filtered []string
	for _, p := range params {
		switch {
		case strings.HasPrefix(p, logLevelParam):
			parameters.LogLevel = strings.TrimPrefix(p, logLevelParam)
		case strings.HasPrefix(p, logFormatParam):
			parameters.LogFormat = strings.TrimPrefix(p, logFormatParam)
		default:
			filtered = append(filtered, p)
		}
	}

	params = filtered

	cmdLineIndex := indexOf(params, cmdLineParam)
	watchedDirIndex := indexOf(params, watchedDirParam)

	if cmdLineIndex == -1 && watchedDirIndex == -1 && len(params) > 1 {
		parameters.CmdLineArgs = params[1:]
	}

	if watchedDirIndex > -1 {
		watchedDirStr := params[watchedDirIndex]
		watchedDirStr = strings.TrimPrefix(watchedDirStr, watchedDirParam)
		watchedDirStr = strings.TrimSuffix(watchedDirStr, "/")
		parameters.WatchedDir = watchedDirStr
	}

	if cmdLineIndex == -1 && watchedDirIndex != -1 {
		beforeWatched := params[1:watchedDirIndex]
		afterWatched := params[watchedDirIndex+1:]
		cmdLineArgs := make([]string, 0, len(beforeWatched)+len(afterWatched))
		cmdLineArgs = append(cmdLineArgs, beforeWatched...)
		cmdLineArgs = append(cmdLineArgs, afterWatched...)
		parameters.CmdLineArgs = cmdLineArgs
	}

	if cmdLineIndex != -1 && cmdLineIndex < len(params) {
		cmdLineStr := strings.TrimPrefix(params[cmdLineIndex], cmdLineParam)
		parameters.CmdLine = cmdLineStr

		if watchedDirIndex > cmdLineIndex {
			parameters.CmdLineArgs = params[cmdLineIndex+1 : watchedDirIndex]
		} else {
			parameters.CmdLineArgs = params[cmdLineIndex+1:]
		}
	}

	return parameters
}

func indexOf(params []string, str string) int {
	for i, arg := range params {
		if strings.Contains(arg, str) {
			return i
		}
	}

	return -1
}
