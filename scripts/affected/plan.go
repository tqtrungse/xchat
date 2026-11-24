/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// taskNamePattern accepts a task name. The task names go into shell commands,
// so the pattern does not accept other characters.
var taskNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.:-]*$`)

// toolTasks are the task names that also run on the module of this command.
// The go command has a subcommand with each of these names.
var toolTasks = map[string]struct{}{
	"build": {},
	"test":  {},
	"vet":   {},
}

// taskList holds the tasks of the root Taskfile and of its includes.
type taskList struct {
	// dirByNamespace maps a namespace to the directory of its Taskfile. The
	// directory is relative to the root and uses "/" as the separator. The
	// tasks of the root Taskfile have no namespace.
	dirByNamespace map[string]string

	// names holds the full name of each task, for example "iam:test".
	names map[string]struct{}
}

// plan holds the result of buildPlan.
type plan struct {
	// commands are the shell commands in the order to run them.
	commands []string

	// skippedTasks are the full names of the tasks that do not exist, for
	// example "pkg:json".
	skippedTasks []string

	// uncoveredModuleDirs are the affected modules that no namespace contains.
	uncoveredModuleDirs []string
}

// parseTaskNames splits the comma-separated task names. It removes the empty
// names and keeps the order.
func parseTaskNames(value string) ([]string, error) {
	var taskNames []string
	for taskName := range strings.SplitSeq(value, ",") {
		taskName = strings.TrimSpace(taskName)
		if taskName == "" {
			continue
		}
		if !taskNamePattern.MatchString(taskName) {
			return nil, fmt.Errorf("task name %q is not valid", taskName)
		}
		taskNames = append(taskNames, taskName)
	}
	if len(taskNames) == 0 {
		return nil, fmt.Errorf("no task name in %q", value)
	}
	return taskNames, nil
}

// loadTaskList reads the tasks with "task --list-all --json" at root.
func loadTaskList(root string) (taskList, error) {
	out, err := run(root, nil, "task", "--list-all", "--json")
	if err != nil {
		return taskList{}, err
	}
	return parseTaskList(root, out)
}

// parseTaskList parses the output of "task --list-all --json".
func parseTaskList(root, out string) (taskList, error) {
	var listing struct {
		Tasks []struct {
			Name     string
			Location struct {
				Taskfile string
			}
		}
	}
	err := json.Unmarshal([]byte(out), &listing)
	if err != nil {
		return taskList{}, fmt.Errorf("decode the task list: %w", err)
	}

	tasks := taskList{
		dirByNamespace: make(map[string]string),
		names:          make(map[string]struct{}, len(listing.Tasks)),
	}
	for _, task := range listing.Tasks {
		tasks.names[task.Name] = struct{}{}

		namespace, _, hasNamespace := strings.Cut(task.Name, ":")
		if !hasNamespace {
			continue
		}

		var taskfileDir string
		taskfileDir, err = filepath.Rel(root, filepath.Dir(task.Location.Taskfile))
		if err != nil {
			return taskList{}, fmt.Errorf("Taskfile of %s: %w", task.Name, err)
		}

		// A task of the root Taskfile, such as infra:up, has a colon in its
		// name, but it is not in a namespace.
		if taskfileDir == "." {
			continue
		}
		tasks.dirByNamespace[namespace] = filepath.ToSlash(taskfileDir)
	}
	return tasks, nil
}

// moduleDirsOf returns the sorted directories of the modules of targets.
func moduleDirsOf(targets []string, modulePathByDir map[string]string) []string {
	moduleDirs := make(map[string]struct{})
	for _, target := range targets {
		importPath := strings.TrimSuffix(target, "/...")
		var owner string
		for moduleDir, modulePath := range modulePathByDir {
			matches := importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
			if matches && len(modulePath) > len(modulePathByDir[owner]) {
				owner = moduleDir
			}
		}
		if owner != "" {
			moduleDirs[owner] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(moduleDirs))
}

// buildPlan returns the shell commands that run each task in taskNames on the
// modules in moduleDirs. The commands keep the order of taskNames. For one task
// name, the namespaces are in sorted order.
func buildPlan(taskNames []string, moduleDirs []string, tasks taskList, toolChanged bool) plan {
	namespaces := slices.Sorted(maps.Keys(tasks.dirByNamespace))

	var result plan
	covered := make(map[string]struct{})
	for _, taskName := range taskNames {
		if _, exists := toolTasks[taskName]; exists && toolChanged {
			result.commands = append(result.commands, "GOWORK=off go -C "+strings.TrimSuffix(toolDir, "/")+" "+taskName+" ./...")
		}

		for _, namespace := range namespaces {
			namespaceDir := tasks.dirByNamespace[namespace]
			isModule, members := namespaceModules(namespaceDir, moduleDirs)
			if !isModule && len(members) == 0 {
				continue
			}
			if isModule {
				covered[namespaceDir] = struct{}{}
			}
			for _, member := range members {
				covered[namespaceDir+"/"+member] = struct{}{}
			}

			qualifiedName := namespace + ":" + taskName
			if _, exists := tasks.names[qualifiedName]; !exists {
				result.skippedTasks = append(result.skippedTasks, qualifiedName)
				continue
			}
			command := "task " + qualifiedName
			if !isModule {
				command += ` MODULES="` + strings.Join(members, " ") + `"`
			}
			result.commands = append(result.commands, command)
		}
	}

	for _, moduleDir := range moduleDirs {
		if _, exists := covered[moduleDir]; !exists {
			result.uncoveredModuleDirs = append(result.uncoveredModuleDirs, moduleDir)
		}
	}
	return result
}

// namespaceModules reports whether namespaceDir is one of moduleDirs. It also
// returns the modules in moduleDirs under namespaceDir, relative to it. If
// namespaceDir is a module, the task of the namespace runs on that module
// only, so the modules under it are not returned.
func namespaceModules(namespaceDir string, moduleDirs []string) (bool, []string) {
	if slices.Contains(moduleDirs, namespaceDir) {
		return true, nil
	}
	var members []string
	for _, moduleDir := range moduleDirs {
		if strings.HasPrefix(moduleDir, namespaceDir+"/") {
			members = append(members, moduleDir[len(namespaceDir)+1:])
		}
	}
	return false, members
}
