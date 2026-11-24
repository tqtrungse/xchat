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

// testModules has two modules and one module whose directory is a prefix of
// another directory name.
func testModules() map[string]string {
	return map[string]string{
		"pkg/log":      "xchat/pkg/log",
		"pkg/logx":     "xchat/pkg/logx",
		"services/iam": "xchat/services/iam",
	}
}

func TestOwnerModule(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "file in a module",
			filePath: "pkg/log/config.go",
			want:     "pkg/log",
		},
		{
			name:     "directory name with a module prefix",
			filePath: "pkg/logx/a.go",
			want:     "pkg/logx",
		},
		{
			name:     "file outside all modules",
			filePath: "scripts/local/iam.sh",
			want:     "",
		},
		{
			name:     "module directory without a slash",
			filePath: "pkg/log",
			want:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := ownerModule(test.filePath, testModules())
			if got != test.want {
				t.Fatalf("ownerModule(%q) = %q, want %q", test.filePath, got, test.want)
			}
		})
	}
}

func TestSelectsAllModules(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		changed []string
		want    bool
	}{
		{
			name:    "go.work",
			changed: []string{"go.work"},
			want:    true,
		},
		{
			name:    "go.work.sum",
			changed: []string{"go.work.sum"},
			want:    true,
		},
		{
			name:    "file of this command",
			changed: []string{"scripts/affected/main.go"},
			want:    true,
		},
		{
			name:    "go.mod of a module",
			changed: []string{"pkg/log/config.go", "services/iam/go.mod"},
			want:    true,
		},
		{
			name:    "go.sum of a module",
			changed: []string{"services/iam/go.sum"},
			want:    false,
		},
		{
			name:    "go.mod in a subdirectory of a module",
			changed: []string{"services/iam/internal/go.mod"},
			want:    false,
		},
		{
			name:    "file outside all modules",
			changed: []string{"scripts/local/iam.sh", "README.md"},
			want:    false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := selectsAllModules(test.changed, testModules())
			if got != test.want {
				t.Fatalf("selectsAllModules(%q) = %v, want %v", test.changed, got, test.want)
			}
		})
	}
}

func TestChangesTool(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		changed []string
		want    bool
	}{
		{
			name:    "file of this command",
			changed: []string{"pkg/log/config.go", "scripts/affected/main_test.go"},
			want:    true,
		},
		{
			name:    "directory name with the prefix of this command",
			changed: []string{"scripts/affectedx/main.go"},
			want:    false,
		},
		{
			name:    "other script",
			changed: []string{"scripts/local/iam.sh"},
			want:    false,
		},
		{
			name:    "no change",
			changed: nil,
			want:    false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := changesTool(test.changed)
			if got != test.want {
				t.Fatalf("changesTool(%q) = %v, want %v", test.changed, got, test.want)
			}
		})
	}
}

func TestModuleFilesOf(t *testing.T) {
	t.Parallel()

	got := moduleFilesOf(
		[]string{
			"README.md",
			"pkg/log/README.md",
			"pkg/log/config.go",
			"scripts/local/iam.sh",
			"services/iam/db/schemas/users.schema",
		},
		testModules(),
	)
	want := []string{
		"pkg/log/config.go",
		"services/iam/db/schemas/users.schema",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("moduleFilesOf() = %q, want %q", got, want)
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()

	importsByDir := map[string][]string{
		"pkg/log":                  nil,
		"services/iam/cmd":         {"xchat/pkg/log"},
		"services/iam/internal/db": nil,
	}

	for _, test := range []struct {
		name         string
		files        []string
		wantFull     []string
		wantGtaFiles []string
	}{
		{
			name:         "Go file in a package goes to gta",
			files:        []string{"pkg/log/config.go"},
			wantGtaFiles: []string{"pkg/log/config.go"},
		},
		{
			name:     "go.sum selects the module",
			files:    []string{"services/iam/go.sum"},
			wantFull: []string{"services/iam"},
		},
		{
			name:     "file outside a package selects the module",
			files:    []string{"services/iam/db/schemas/users.schema"},
			wantFull: []string{"services/iam"},
		},
		{
			name:     "testdata selects the module",
			files:    []string{"pkg/log/testdata/a.txt"},
			wantFull: []string{"pkg/log"},
		},
		{
			name:     "file of a deleted package selects the module",
			files:    []string{"pkg/log/deleted/a.go"},
			wantFull: []string{"pkg/log"},
		},
		{
			name: "both kinds",
			files: []string{
				"pkg/log/config.go",
				"services/iam/go.sum",
				"services/iam/cmd/main.go",
			},
			wantFull: []string{"services/iam"},
			wantGtaFiles: []string{
				"pkg/log/config.go",
				"services/iam/cmd/main.go",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fullModuleDirs, gtaFiles := classify(test.files, testModules(), importsByDir)
			gotFull := slices.Sorted(maps.Keys(fullModuleDirs))
			gotGtaFiles := slices.Sorted(maps.Keys(gtaFiles))
			if !slices.Equal(gotFull, test.wantFull) {
				t.Fatalf("full modules = %q, want %q", gotFull, test.wantFull)
			}
			if !slices.Equal(gotGtaFiles, test.wantGtaFiles) {
				t.Fatalf("gta files = %q, want %q", gotGtaFiles, test.wantGtaFiles)
			}
		})
	}
}

func TestImporterDirs(t *testing.T) {
	t.Parallel()

	importsByDir := map[string][]string{
		"pkg/jwt":          {"fmt", "xchat/pkg/jitter"},
		"pkg/database":     {"xchat/pkg/jitter/backoff"},
		"pkg/log":          {"xchat/pkg/jitterx"},
		"services/iam/cmd": {"xchat/pkg/log"},
	}

	got := importerDirs(importsByDir, []string{"xchat/pkg/jitter"})
	want := []string{
		"pkg/database",
		"pkg/jwt",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("importerDirs() = %q, want %q", got, want)
	}
}

func TestParsePackages(t *testing.T) {
	t.Parallel()

	out := "/repo/pkg/log\txchat/pkg/pool  github.com/stretchr/testify/require\n" +
		"/repo/pkg/log/logtest\t  \n" +
		"\n"

	got, err := parsePackages("/repo", out)
	if err != nil {
		t.Fatalf("parsePackages() error = %v", err)
	}
	want := map[string][]string{
		"pkg/log":         {"xchat/pkg/pool", "github.com/stretchr/testify/require"},
		"pkg/log/logtest": {},
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("parsePackages() = %q, want %q", got, want)
	}
}

func TestSplitNull(t *testing.T) {
	t.Parallel()

	got := splitNull("b.go\x00a b.go\x00\x00b.go\x00")
	want := []string{
		"a b.go",
		"b.go",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("splitNull() = %q, want %q", got, want)
	}
}

func TestBuildTargets(t *testing.T) {
	t.Parallel()

	got := buildTargets(
		testModules(),
		setOf("services/iam"),
		[]string{
			"xchat/services/iam/cmd",
			"xchat/pkg/logx",
			"xchat/pkg/log/logtest",
		},
	)
	want := []string{
		"xchat/pkg/log/logtest",
		"xchat/pkg/logx",
		"xchat/services/iam/...",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("buildTargets() = %q, want %q", got, want)
	}
}
