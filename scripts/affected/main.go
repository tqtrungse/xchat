/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

// Command affected prints the Go packages that a diff affects.
//
// The diff compares the merge base of the base branch and HEAD with the
// working tree. Thus, it contains the commits of the branch, the staged and
// unstaged changes, and the untracked files. A clean checkout on CI has only
// the commits.
//
// Each output line is one argument for a go command that runs at the
// repository root. A line is an import path, or a module pattern such as
// xchat/services/iam/... . The output is empty if no package is affected.
//
// gta finds the packages that the changed Go packages affect. These include the
// packages that import a changed package only in their tests. gta does not see
// some changes, so affected adds these rules:
//
//   - A change to go.work, go.work.sum, a go.mod, or this command selects all
//     modules. The workspace selects one version of each dependency for all
//     modules, so a go.mod change can change the build of each module.
//   - A change to go.sum of a module selects all of that module.
//   - A change to a file outside a Go package of a module selects all of that
//     module. Examples are db/schemas, testdata, and a deleted package.
//   - When affected selects all of a module, it also selects the packages that
//     import a package of that module, and their dependents.
//   - A change to a Markdown file has no effect.
//   - A change to a file outside all modules has no effect. An example is
//     scripts/local/iam.sh.
//
// This module is not in go.work, so that its dependencies stay out of the
// workspace. Run it from the repository root:
//
//	GOWORK=off go -C scripts/affected run . [-base origin/master]
//
// With -tasks lint,vet,test, affected prints one shell command on each line
// instead. For each task name in the given order, it gives these commands:
//
//   - "task ns:name" for each Taskfile namespace whose directory is an
//     affected module, for example "task iam:lint".
//   - "task ns:name MODULES=..." for each Taskfile namespace whose directory
//     contains affected modules, for example task pkg:lint MODULES="jwt log".
//     The Taskfile must use MODULES to select its modules.
//   - "GOWORK=off go -C scripts/affected name ./..." if the diff changes this
//     command and name is build, vet, or test. The go command at the root does
//     not see this module, because it is not in go.work.
//
// affected reads the namespaces from "task --list-all --json". A namespace
// without the task is skipped with a message on the standard error.
//
// It requires gta: go install github.com/digitalocean/gta/cmd/gta@v0.2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const (
	ignoredSuffix = ".md"

	// toolDir is the directory of this command, relative to the root. A change in
	// it selects all modules, because the rules can change.
	toolDir = "scripts/affected/"

	// packageTemplate gives one line for each package: the directory, a tab, and
	// the imports. The imports include the imports of the test files.
	packageTemplate = `{{.Dir}}{{"\t"}}{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`
)

var (
	// globalFiles are relative to the root. A change to one of them selects all
	// modules.
	globalFiles = map[string]struct{}{
		"go.work":     {},
		"go.work.sum": {},
	}

	// globalModuleFiles are relative to a module. A change to one of them selects
	// all modules.
	globalModuleFiles = map[string]struct{}{
		"go.mod": {},
	}

	// moduleFiles are relative to a module. A change to one of them selects that
	// module.
	moduleFiles = map[string]struct{}{
		"go.sum": {},
	}
)

func main() {
	base := flag.String("base", "origin/master", "branch to diff against")
	tasks := flag.String("tasks", "", "comma-separated task names; print the shell commands that run them")
	flag.Parse()
	if flag.NArg() > 0 {
		exitWithError(fmt.Errorf("unexpected arguments: %q", flag.Args()))
	}

	lines, err := execute(*base, *tasks)
	if err != nil {
		exitWithError(err)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}

func exitWithError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "affected: %v\n", err)
	os.Exit(1)
}

// execute returns the output lines. If tasks is empty, the lines are the
// targets. Otherwise, the lines are the shell commands that run the tasks.
func execute(base, tasks string) ([]string, error) {
	var (
		taskNames []string
		err       error
	)
	if tasks != "" {
		// Validate the names first, so that a wrong name fails before the
		// slow steps.
		taskNames, err = parseTaskNames(tasks)
		if err != nil {
			return nil, err
		}
	}

	root, err := findRoot()
	if err != nil {
		return nil, err
	}

	goEnv := workspaceEnv(root)
	modulePathByDir, err := loadModules(root, goEnv)
	if err != nil {
		return nil, err
	}

	changed, err := changedFiles(root, base)
	if err != nil {
		return nil, err
	}

	targets, err := affectedTargets(root, goEnv, modulePathByDir, changed)
	if err != nil {
		return nil, err
	}
	if tasks == "" {
		return targets, nil
	}

	moduleDirs := moduleDirsOf(targets, modulePathByDir)
	if len(moduleDirs) == 0 && !changesTool(changed) {
		return nil, nil
	}

	tasksList, err := loadTaskList(root)
	if err != nil {
		return nil, err
	}

	pl := buildPlan(taskNames, moduleDirs, tasksList, changesTool(changed))
	for _, skippedTask := range pl.skippedTasks {
		_, _ = fmt.Fprintf(os.Stderr, "affected: skip %s: the task does not exist\n", skippedTask)
	}
	for _, moduleDir := range pl.uncoveredModuleDirs {
		_, _ = fmt.Fprintf(os.Stderr, "affected: skip module %s: no Taskfile namespace contains it\n", moduleDir)
	}
	return pl.commands, nil
}

// changesTool reports whether changed contains a file of this command.
func changesTool(changed []string) bool {
	return slices.ContainsFunc(changed, func(changedFile string) bool {
		return strings.HasPrefix(changedFile, toolDir)
	})
}

// affectedTargets returns the sorted targets for the changed files.
func affectedTargets(
	root string,
	goEnv []string,
	modulePathByDir map[string]string,
	changed []string,
) ([]string, error) {
	allModules := buildTargets(modulePathByDir, setOf(slices.Collect(maps.Keys(modulePathByDir))...), nil)
	if selectsAllModules(changed, modulePathByDir) {
		return allModules, nil
	}

	files := moduleFilesOf(changed, modulePathByDir)
	if len(files) == 0 {
		return nil, nil
	}

	importsByDir, err := loadPackages(root, goEnv, slices.Collect(maps.Values(modulePathByDir)))
	if err != nil {
		return nil, err
	}
	fullModuleDirs, gtaFiles := classify(files, modulePathByDir, importsByDir)

	if len(fullModuleDirs) > 0 {
		var fullModulePaths []string
		for moduleDir := range fullModuleDirs {
			fullModulePaths = append(fullModulePaths, modulePathByDir[moduleDir])
		}
		for _, importerDir := range importerDirs(importsByDir, fullModulePaths) {
			goFile, exists := firstGoFile(filepath.Join(root, importerDir))
			if exists {
				gtaFiles[path.Join(importerDir, goFile)] = struct{}{}
			}
		}
	}

	gtaPackages, err := runGta(root, goEnv, slices.Sorted(maps.Keys(gtaFiles)))
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("%w. Install it: go install github.com/digitalocean/gta/cmd/gta@v0.2.0", err)
	}
	if err != nil {
		// gta stops when a package cannot load, for example when two files in
		// one directory declare different package names. Select the full
		// module, so that the go command reports the error.
		_, _ = fmt.Fprintf(os.Stderr, "affected: gta failed, so the full modules are selected: %v\n", err)
		gtaPackages = nil
		for gtaFile := range gtaFiles {
			fullModuleDirs[ownerModule(gtaFile, modulePathByDir)] = struct{}{}
		}
	}
	return buildTargets(modulePathByDir, fullModuleDirs, gtaPackages), nil
}

// run runs a command in dir and returns its standard output. If env is nil,
// the command gets the environment of this process.
func run(dir string, env []string, name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = env
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("%s is not installed: %w", name, err)
	}
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// findRoot return absolute path for current folder.
func findRoot() (string, error) {
	out, err := run("", nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(strings.TrimSpace(out))
}

// workspaceEnv gives the environment for the go commands and for gta. The
// task runs this command with GOWORK=off, so the environment must set GOWORK
// to the go.work of the root again.
func workspaceEnv(root string) []string {
	// Get list setup variables and delete variable with name "GOWORK=" from list.
	env := slices.DeleteFunc(os.Environ(), func(variable string) bool {
		return strings.HasPrefix(variable, "GOWORK=")
	})
	// Append "GOWORK=" with path to the go.work file within the root directory
	return append(env, "GOWORK="+filepath.Join(root, "go.work"))
}

// loadModules returns a map from the module directory to the module path. The
// directory is relative to root and uses "/" as the separator.
func loadModules(root string, goEnv []string) (map[string]string, error) {
	out, err := run(root, goEnv, "go", "work", "edit", "-json")
	if err != nil {
		return nil, err
	}

	var work struct {
		Use []struct {
			DiskPath string
		}
	}
	err = json.Unmarshal([]byte(out), &work)
	if err != nil {
		return nil, fmt.Errorf("decode go.work: %w", err)
	}

	modulePathByDir := make(map[string]string, len(work.Use))
	for _, use := range work.Use {
		moduleDir := path.Clean(filepath.ToSlash(use.DiskPath))
		out, err = run(root, goEnv, "go", "mod", "edit", "-json", path.Join(moduleDir, "go.mod"))
		if err != nil {
			return nil, err
		}

		var goMod struct {
			Module struct {
				Path string
			}
		}
		err = json.Unmarshal([]byte(out), &goMod)
		if err != nil {
			return nil, fmt.Errorf("decode %s/go.mod: %w", moduleDir, err)
		}

		modulePathByDir[moduleDir] = goMod.Module.Path
	}
	return modulePathByDir, nil
}

// changedFiles returns the changed paths relative to root. The diff starts at
// the merge base. Thus, a commit on base after the branch point does not count
// as a change of the branch.
func changedFiles(root, base string) ([]string, error) {
	// if new commits have been added to `base` since you created your branch,
	// the `merge-base` remains the commit from the point when you branched off;
	// this ensures that the new commits on `main` are not counted as part of your own changes.
	out, err := run(root, nil, "git", "merge-base", base, "HEAD")
	if err != nil {
		return nil, fmt.Errorf(
			"cannot find the merge base of %s and HEAD. On CI, fetch %s and the full history (fetch-depth: 0): %w",
			base, base, err,
		)
	}
	mergeBase := strings.TrimSpace(out)

	tracked, err := run(root, nil, "git", "diff", "-z", "--name-only", "--no-renames", mergeBase)
	if err != nil {
		return nil, err
	}

	untracked, err := run(root, nil, "git", "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	return splitNull(tracked + untracked), nil
}

// splitNull splits the output of a git command with -z. It removes the empty
// and the repeated paths.
func splitNull(out string) []string {
	paths := setOf(strings.Split(out, "\x00")...)
	delete(paths, "")
	return slices.Sorted(maps.Keys(paths))
}

// selectsAllModules reports whether a change in changed selects all modules.
func selectsAllModules(changed []string, modulePathByDir map[string]string) bool {
	if changesTool(changed) {
		return true
	}
	for _, changedFile := range changed {
		if _, exists := globalFiles[changedFile]; exists {
			return true
		}

		moduleDir := ownerModule(changedFile, modulePathByDir)
		if moduleDir == "" {
			continue
		}
		if _, exists := globalModuleFiles[changedFile[len(moduleDir)+1:]]; exists {
			return true
		}
	}
	return false
}

// moduleFilesOf returns the paths in changed that are in a module. It removes
// the Markdown files.
func moduleFilesOf(changed []string, modulePathByDir map[string]string) []string {
	var files []string
	for _, changedFile := range changed {
		if strings.HasSuffix(changedFile, ignoredSuffix) {
			continue
		}
		if ownerModule(changedFile, modulePathByDir) != "" {
			files = append(files, changedFile)
		}
	}
	return files
}

// ownerModule returns the directory of the innermost module that contains
// filePath. It returns "" if no module contains filePath.
func ownerModule(filePath string, modulePathByDir map[string]string) string {
	var owner string
	for moduleDir := range modulePathByDir {
		if strings.HasPrefix(filePath, moduleDir+"/") && len(moduleDir) > len(owner) {
			owner = moduleDir
		}
	}
	return owner
}

// loadPackages returns a map from the directory of each Go package to its
// imports. The directory is relative to root.
func loadPackages(root string, goEnv []string, modulePaths []string) (map[string][]string, error) {
	patterns := make([]string, 0, len(modulePaths))
	for _, modulePath := range modulePaths {
		patterns = append(patterns, modulePath+"/...")
	}
	// -e lists a package that has an error. A syntax error must not stop this
	// command, because the go command reports it later.
	args := append([]string{"list", "-e", "-f", packageTemplate}, patterns...)
	out, err := run(root, goEnv, "go", args...)
	if err != nil {
		return nil, err
	}
	return parsePackages(root, out)
}

// parsePackages parses the output of go list with packageTemplate.
func parsePackages(root, out string) (map[string][]string, error) {
	importsByDir := make(map[string][]string)
	for line := range strings.Lines(out) {
		packageDir, imports, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if packageDir == "" {
			continue
		}
		relativeDir, err := filepath.Rel(root, packageDir)
		if err != nil {
			return nil, fmt.Errorf("package directory %s: %w", packageDir, err)
		}
		importsByDir[filepath.ToSlash(relativeDir)] = strings.Fields(imports)
	}
	return importsByDir, nil
}

// classify returns the modules to select in full, and the files to give to
// gta. Each path in files must be in a module.
func classify(
	files []string,
	modulePathByDir map[string]string,
	importsByDir map[string][]string,
) (map[string]struct{}, map[string]struct{}) {
	fullModuleDirs := make(map[string]struct{})
	gtaFiles := make(map[string]struct{})
	for _, changedFile := range files {
		moduleDir := ownerModule(changedFile, modulePathByDir)
		relativeFile := changedFile[len(moduleDir)+1:]

		_, isModuleFile := moduleFiles[relativeFile]
		_, inPackage := importsByDir[path.Dir(changedFile)]
		inTestdata := slices.Contains(strings.Split(relativeFile, "/"), "testdata")

		if isModuleFile || !inPackage || inTestdata {
			fullModuleDirs[moduleDir] = struct{}{}
			continue
		}
		gtaFiles[changedFile] = struct{}{}
	}
	return fullModuleDirs, gtaFiles
}

// importerDirs returns the sorted directories of the packages that import a
// package of a module in modulePaths. The import path of a deleted package also
// matches, so the go command reports the importers of a deleted package.
func importerDirs(importsByDir map[string][]string, modulePaths []string) []string {
	var dirs []string
	for packageDir, imports := range importsByDir {
		if slices.ContainsFunc(imports, func(importPath string) bool {
			return inModule(importPath, modulePaths)
		}) {
			dirs = append(dirs, packageDir)
		}
	}
	slices.Sort(dirs)
	return dirs
}

// inModule reports whether importPath is a package of a module in modulePaths.
func inModule(importPath string, modulePaths []string) bool {
	return slices.ContainsFunc(modulePaths, func(modulePath string) bool {
		return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
	})
}

// firstGoFile returns the name of the first Go file in dir. gta finds the
// package of a file from its directory, so one file is sufficient.
func firstGoFile(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return entry.Name(), true
		}
	}
	return "", false
}

// runGta returns the import paths of the packages that files affect. The paths
// in files are relative to root.
func runGta(root string, goEnv []string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}

	changedList, err := os.CreateTemp("", "affected-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create the changed file list: %w", err)
	}
	defer func() {
		_ = os.Remove(changedList.Name())
	}()

	// gta accepts only absolute paths in the file of -changed-files.
	var content strings.Builder
	for _, changedFile := range files {
		content.WriteString(filepath.Join(root, filepath.FromSlash(changedFile)))
		content.WriteString("\n")
	}

	_, err = changedList.WriteString(content.String())
	if err != nil {
		_ = changedList.Close()
		return nil, fmt.Errorf("write the changed file list: %w", err)
	}

	err = changedList.Close()
	if err != nil {
		return nil, fmt.Errorf("close the changed file list: %w", err)
	}

	out, err := run(root, goEnv, "gta", "-changed-files", changedList.Name())
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// buildTargets returns the sorted targets. A module in fullModuleDirs gives
// one module pattern. A package in packages gives one import path, unless its
// module is in fullModuleDirs.
func buildTargets(
	modulePathByDir map[string]string,
	fullModuleDirs map[string]struct{},
	packages []string,
) []string {
	fullModulePaths := make([]string, 0, len(fullModuleDirs))
	targets := make(map[string]struct{})
	for moduleDir := range fullModuleDirs {
		modulePath := modulePathByDir[moduleDir]
		fullModulePaths = append(fullModulePaths, modulePath)
		targets[modulePath+"/..."] = struct{}{}
	}
	for _, importPath := range packages {
		if !inModule(importPath, fullModulePaths) {
			targets[importPath] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(targets))
}

func setOf(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
