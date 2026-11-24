/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"maps"
	"slices"
	"testing"
)

func TestParseTaskNames(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:  "one name",
			value: "test",
			want:  []string{"test"},
		},
		{
			name:  "names keep the order",
			value: "lint, vet,,test",
			want:  []string{"lint", "vet", "test"},
		},
		{
			name:  "name with a colon",
			value: "tidy:check",
			want:  []string{"tidy:check"},
		},
		{
			name:    "shell characters",
			value:   "test;rm -rf /",
			wantErr: true,
		},
		{
			name:    "go flags",
			value:   "test -race",
			wantErr: true,
		},
		{
			name:    "only commas",
			value:   ",,",
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseTaskNames(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseTaskNames(%q) = %q, want an error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTaskNames(%q) error = %v", test.value, err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("parseTaskNames(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestParseTaskList(t *testing.T) {
	t.Parallel()

	out := `{"tasks": [
		{"name": "affected", "location": {"taskfile": "/repo/Taskfile.yaml"}},
		{"name": "infra:up", "location": {"taskfile": "/repo/Taskfile.yaml"}},
		{"name": "iam:test", "location": {"taskfile": "/repo/services/iam/Taskfile.yaml"}},
		{"name": "pkg:vet", "location": {"taskfile": "/repo/pkg/Taskfile.yaml"}}
	]}`

	got, err := parseTaskList("/repo", out)
	if err != nil {
		t.Fatalf("parseTaskList() error = %v", err)
	}
	wantDirs := map[string]string{
		"iam": "services/iam",
		"pkg": "pkg",
	}
	if !maps.Equal(got.dirByNamespace, wantDirs) {
		t.Fatalf("namespaces = %v, want %v", got.dirByNamespace, wantDirs)
	}
	wantNames := []string{"affected", "iam:test", "infra:up", "pkg:vet"}
	if gotNames := slices.Sorted(maps.Keys(got.names)); !slices.Equal(gotNames, wantNames) {
		t.Fatalf("names = %q, want %q", gotNames, wantNames)
	}
}

func TestModuleDirsOf(t *testing.T) {
	t.Parallel()

	got := moduleDirsOf(
		[]string{
			"xchat/pkg/log/logtest",
			"xchat/pkg/logx",
			"xchat/services/iam/...",
			"xchat/services/iam/cmd",
			"github.com/other/module",
		},
		testModules(),
	)
	want := []string{
		"pkg/log",
		"pkg/logx",
		"services/iam",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("moduleDirsOf() = %q, want %q", got, want)
	}
}

// testTaskList has one namespace for one module, one namespace for several
// modules, and one namespace without a vet task.
func testTaskList() taskList {
	return taskList{
		dirByNamespace: map[string]string{
			"iam":    "services/iam",
			"pkg":    "pkg",
			"shared": "services/internal/shared",
		},
		names: setOf(
			"iam:lint",
			"iam:test",
			"iam:vet",
			"pkg:lint",
			"pkg:test",
			"pkg:vet",
			"shared:lint",
			"shared:test",
		),
	}
}

func TestBuildPlan(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		taskNames     []string
		moduleDirs    []string
		toolChanged   bool
		wantCommands  []string
		wantSkipped   []string
		wantUncovered []string
	}{
		{
			name:       "tasks keep the order and namespaces are sorted",
			taskNames:  []string{"lint", "test"},
			moduleDirs: []string{"pkg/jwt", "pkg/log", "services/iam"},
			wantCommands: []string{
				"task iam:lint",
				`task pkg:lint MODULES="jwt log"`,
				"task iam:test",
				`task pkg:test MODULES="jwt log"`,
			},
		},
		{
			name:         "namespace without the task is skipped",
			taskNames:    []string{"vet"},
			moduleDirs:   []string{"services/iam", "services/internal/shared"},
			wantCommands: []string{"task iam:vet"},
			wantSkipped:  []string{"shared:vet"},
		},
		{
			name:        "changed tool runs its go command first",
			taskNames:   []string{"lint", "test"},
			moduleDirs:  []string{"services/iam"},
			toolChanged: true,
			wantCommands: []string{
				"task iam:lint",
				"GOWORK=off go -C scripts/affected test ./...",
				"task iam:test",
			},
		},
		{
			name:         "changed tool without affected modules",
			taskNames:    []string{"vet"},
			toolChanged:  true,
			wantCommands: []string{"GOWORK=off go -C scripts/affected vet ./..."},
		},
		{
			name:          "module outside all namespaces",
			taskNames:     []string{"test"},
			moduleDirs:    []string{"services/chat", "services/iam"},
			wantCommands:  []string{"task iam:test"},
			wantUncovered: []string{"services/chat"},
		},
		{
			name:       "no affected module",
			taskNames:  []string{"test"},
			moduleDirs: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := buildPlan(test.taskNames, test.moduleDirs, testTaskList(), test.toolChanged)
			if !slices.Equal(got.commands, test.wantCommands) {
				t.Fatalf("commands = %q, want %q", got.commands, test.wantCommands)
			}
			if !slices.Equal(got.skippedTasks, test.wantSkipped) {
				t.Fatalf("skipped tasks = %q, want %q", got.skippedTasks, test.wantSkipped)
			}
			if !slices.Equal(got.uncoveredModuleDirs, test.wantUncovered) {
				t.Fatalf("uncovered modules = %q, want %q", got.uncoveredModuleDirs, test.wantUncovered)
			}
		})
	}
}
