// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParams(t *testing.T) {
	defaults := Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{}, WatchedDir: defaultWatchedDir,
		LogLevel: "info", LogFormat: "json"}

	tests := []struct {
		input    []string
		expected Parameters
	}{
		{nil, defaults},
		{[]string{""}, defaults},
		{[]string{"", "60"}, Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{"60"}, WatchedDir: defaultWatchedDir,
			LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "--watched-dir=/tmp"}, Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{}, WatchedDir: "/tmp",
			LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "test", "6565", "--watched-dir=/tmp"}, Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{"test", "6565"},
			WatchedDir: "/tmp", LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "--watched-dir=/tmp", "test", "6565"}, Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{"test", "6565"},
			WatchedDir: "/tmp", LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "--watched-dir=/tmp", "--cmd-line=sleep", "60"}, Parameters{CmdLine: "sleep", CmdLineArgs: []string{"60"},
			WatchedDir: "/tmp", LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "--cmd-line=sleep", "60", "--watched-dir=/tmp"}, Parameters{CmdLine: "sleep", CmdLineArgs: []string{"60"},
			WatchedDir: "/tmp", LogLevel: "info", LogFormat: "json"}},
		{[]string{"", "--log-level=debug", "--log-format=text", "--watched-dir=/tmp"}, Parameters{CmdLine: defaultCmdLine, CmdLineArgs: []string{},
			WatchedDir: "/tmp", LogLevel: "debug", LogFormat: "text"}},
	}

	for i, test := range tests {
		t.Logf("running test %d", i)
		assert.EqualValues(t, test.expected, Parse(test.input))
	}
}
