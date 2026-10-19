// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"slices"
	"testing"
)

func TestPackageWatcher(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name: "eval failures are not incomplete",
			output: "--- FAIL: TestReactEval (1.00s)\n" +
				"FAIL\n" +
				"FAIL\texample.com/m/react\t1.234s\n" +
				"ok  \texample.com/m/threads\t2.000s\n",
		},
		{
			name: "timeout panic marks only its package",
			output: "ok  \texample.com/m/react\t1.000s\n" +
				"panic: test timed out after 30m0s\n" +
				"goroutine 1 [running]:\n" +
				"FAIL\texample.com/m/mcpserver\t1800.123s\n" +
				"--- FAIL: TestThreadsEval (1.00s)\n" +
				"FAIL\texample.com/m/threads\t2.000s\n",
			want: []string{"example.com/m/mcpserver"},
		},
		{
			name:   "build failure",
			output: "FAIL\texample.com/m/channels [build failed]\n",
			want:   []string{"example.com/m/channels"},
		},
		{
			name:   "setup failure",
			output: "FAIL\texample.com/m/channels [setup failed]\n",
			want:   []string{"example.com/m/channels"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w packageWatcher
			// Split writes mid-line, as a pipe may deliver them.
			for _, chunk := range []string{tc.output[:len(tc.output)/2], tc.output[len(tc.output)/2:]} {
				if _, err := w.Write([]byte(chunk)); err != nil {
					t.Fatal(err)
				}
			}
			if got := w.packages(); !slices.Equal(got, tc.want) {
				t.Errorf("packages() = %v, want %v", got, tc.want)
			}
		})
	}
}
