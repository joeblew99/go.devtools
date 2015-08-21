// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $V23_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"v.io/x/devtools/internal/collect"
	"v.io/x/devtools/internal/project"
	"v.io/x/devtools/internal/tool"
	"v.io/x/devtools/internal/util"
	"v.io/x/lib/cmdline"
)

var (
	detailedOutputFlag bool
	gotoolsBinPathFlag string

	commentRE = regexp.MustCompile("^($|[:space:]*#)")
)

func init() {
	cmdAPICheck.Flags.BoolVar(&detailedOutputFlag, "detailed", true, "If true, shows each API change in an expanded form. Otherwise, only a summary is shown.")
	cmdAPI.Flags.StringVar(&gotoolsBinPathFlag, "gotools-bin", "", "The path to the gotools binary to use. If empty, gotools will be built if necessary.")

	tool.InitializeProjectFlags(&cmdAPI.Flags)
	tool.InitializeRunFlags(&cmdAPI.Flags)
}

// cmdAPI represents the "v23 api" command.
var cmdAPI = &cmdline.Command{
	Name:     "api",
	Short:    "Manage vanadium public API",
	Long:     "Use this command to ensure that no unintended changes are made to the vanadium public API.",
	Children: []*cmdline.Command{cmdAPICheck, cmdAPIUpdate},
}

// cmdAPICheck represents the "v23 api check" command.
var cmdAPICheck = &cmdline.Command{
	Runner:   cmdline.RunnerFunc(runAPICheck),
	Name:     "check",
	Short:    "Check if any changes have been made to the public API",
	Long:     "Check if any changes have been made to the public API.",
	ArgsName: "<projects>",
	ArgsLong: "<projects> is a list of vanadium projects to check. If none are specified, all projects that require a public API check upon presubmit are checked.",
}

func readAPIFileContents(ctx *tool.Context, path string) (_ []byte, e error) {
	var buf bytes.Buffer
	file, err := ctx.Run().Open(path)
	defer collect.Error(file.Close, &e)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if !commentRE.Match(line) {
			buf.Write(line)
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), err
}

type packageChange struct {
	name          string
	projectName   string
	apiFilePath   string
	oldAPI        map[string]bool // set
	newAPI        map[string]bool // set
	newAPIContent []byte

	// If true, indicates that there was a problem reading the old API file.
	apiFileError error
}

// buildGotools builds the gotools binary and returns the path to the built
// binary and the function to call to clean up the built binary (always
// non-nil). If the binary could not be built, the empty string and a non-nil
// error are returned.
//
// If the gotools_bin flag is specified, that path, a no-op cleanup and a
// nil error are returned.
func buildGotools(ctx *tool.Context) (string, func() error, error) {
	nopCleanup := func() error { return nil }
	if gotoolsBinPathFlag != "" {
		return gotoolsBinPathFlag, nopCleanup, nil
	}

	// Determine the location of the gotools source.
	projects, _, err := project.ReadManifest(ctx)
	if err != nil {
		return "", nopCleanup, err
	}

	project, ok := projects["third_party"]
	if !ok {
		return "", nopCleanup, fmt.Errorf(`project "third_party" not found`)
	}
	newGoPath := filepath.Join(project.Path, "go")

	// Build the gotools binary.
	tempDir, err := ctx.Run().TempDir("", "")
	if err != nil {
		return "", nopCleanup, err
	}
	cleanup := func() error { return ctx.Run().RemoveAll(tempDir) }

	gotoolsBin := filepath.Join(tempDir, "gotools")
	opts := ctx.Run().Opts()
	opts.Env["GOPATH"] = newGoPath
	if err := ctx.Run().CommandWithOpts(opts, "go", "build", "-o", gotoolsBin, "github.com/visualfc/gotools"); err != nil {
		return "", cleanup, err
	}

	return gotoolsBin, cleanup, nil
}

// getCurrentAPI runs the gotools api command against the given directory and
// returns the bytes that should go into the .api file for that directory.
func getCurrentAPI(ctx *tool.Context, gotoolsBin, dir string) ([]byte, error) {
	env, err := util.VanadiumEnvironment(ctx)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdout = &output
	opts.Env = env.ToMap()
	if err := ctx.Run().CommandWithOpts(opts, gotoolsBin, "goapi", dir); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func isFailedAPICheckFatal(projectName string, apiCheckProjects map[string]struct{}, apiFileError error) bool {
	if os.IsNotExist(apiFileError) {
		if _, ok := apiCheckProjects[projectName]; !ok {
			return false
		}
	}
	return true
}

func shouldIgnoreFile(file string) bool {
	if !strings.HasSuffix(file, ".go") {
		return true
	}
	pathComponents := strings.Split(file, string(os.PathSeparator))
	for _, component := range pathComponents {
		if component == "testdata" || component == "internal" {
			return true
		}
	}
	return false
}

func splitLinesToSet(in []byte) map[string]bool {
	result := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(in))
	for scanner.Scan() {
		result[scanner.Text()] = true
	}
	return result
}

func packageName(path string) string {
	components := strings.Split(path, string(os.PathSeparator))
	for i, component := range components {
		if component == "src" {
			return strings.Join(components[i+1:], "/")
		}
	}
	return ""
}

func getPackageChanges(ctx *tool.Context, apiCheckProjects map[string]struct{}, args []string) (changes []packageChange, e error) {
	gotoolsBin, cleanup, err := buildGotools(ctx)
	if err != nil {
		return nil, err
	}
	defer collect.Error(cleanup, &e)
	projects, err := project.ParseNames(ctx, args, apiCheckProjects)
	if err != nil {
		return nil, err
	}
	for name, project := range projects {
		path := project.Path
		branch, err := ctx.Git(tool.RootDirOpt(path)).CurrentBranchName()
		if err != nil {
			return nil, err
		}
		files, err := ctx.Git(tool.RootDirOpt(path)).ModifiedFiles("master", branch)
		if err != nil {
			return nil, err
		}
		// Extract the directories for these files.
		dirs := make(map[string]bool) // set
		for _, file := range files {
			if !shouldIgnoreFile(file) {
				dirs[filepath.Join(path, filepath.Dir(file))] = true
			}
		}
		if len(dirs) == 0 {
			continue
		}
		for dir := range dirs {
			// Read the API state in the working directory.
			currentAPI, err := getCurrentAPI(ctx, gotoolsBin, dir)
			if err != nil {
				return nil, err
			}
			// Read the existing public API file.
			apiFilePath := filepath.Join(dir, ".api")
			apiFileContents, apiFileError := readAPIFileContents(ctx, apiFilePath)
			if apiFileError != nil {
				if os.IsNotExist(apiFileError) && len(currentAPI) == 0 {
					// The API file doesn't exist but the
					// public API in the working directory
					// is empty anyway.
					continue
				}
				if !isFailedAPICheckFatal(name, apiCheckProjects, apiFileError) {
					// We couldn't read the API file, but this project doesn't
					// require one.  Just warn the user.
					fmt.Fprintf(ctx.Stderr(), "WARNING: could not read public API from %s: %v\n", apiFilePath, err)
					fmt.Fprintf(ctx.Stderr(), "WARNING: skipping public API check for %s\n", dir)
					continue
				}
			}
			if apiFileError != nil || !bytes.Equal(currentAPI, apiFileContents) {
				pkgName := packageName(dir)
				if pkgName == "" {
					pkgName = dir
				}
				// The user has changed the public API or we
				// couldn't read the public API in the first
				// place.
				changes = append(changes, packageChange{
					name:          pkgName,
					projectName:   name,
					apiFilePath:   apiFilePath,
					oldAPI:        splitLinesToSet(apiFileContents),
					newAPI:        splitLinesToSet(currentAPI),
					newAPIContent: currentAPI,
					apiFileError:  apiFileError,
				})
			}
		}
	}
	return
}

func runAPICheck(env *cmdline.Env, args []string) error {
	return doAPICheck(env.Stdout, env.Stderr, args, detailedOutputFlag)
}

func printChangeSummary(out io.Writer, change packageChange, detailedOutput bool) {
	var removedEntries []string
	var addedEntries []string
	for entry, _ := range change.oldAPI {
		if !change.newAPI[entry] {
			removedEntries = append(removedEntries, entry)
		}
	}
	for entry, _ := range change.newAPI {
		if !change.oldAPI[entry] {
			addedEntries = append(addedEntries, entry)
		}
	}
	if detailedOutput {
		fmt.Fprintf(out, "Changes for package %s\n", change.name)
		if len(removedEntries) > 0 {
			fmt.Fprintf(out, "The following %d entries were removed:\n", len(removedEntries))
			for _, entry := range removedEntries {
				fmt.Fprintf(out, "\t%s\n", entry)
			}
		}
		if len(addedEntries) > 0 {
			fmt.Fprintf(out, "The following %d entries were added:\n", len(addedEntries))
			for _, entry := range addedEntries {
				fmt.Fprintf(out, "\t%s\n", entry)
			}
		}
	} else {
		fmt.Fprintf(out, "package %s: %d entries removed, %d entries added\n", change.name, len(removedEntries), len(addedEntries))
	}
}

func doAPICheck(stdout, stderr io.Writer, args []string, detailedOutput bool) error {
	ctx := tool.NewContext(tool.ContextOpts{
		Color:    &tool.ColorFlag,
		DryRun:   &tool.DryRunFlag,
		Manifest: &tool.ManifestFlag,
		Verbose:  &tool.VerboseFlag,
		Stdout:   stdout,
		Stderr:   stderr,
	})
	config, err := util.LoadConfig(ctx)
	if err != nil {
		return err
	}
	changes, err := getPackageChanges(ctx, config.APICheckProjects(), args)
	if err != nil {
		return err
	} else if len(changes) > 0 {
		for _, change := range changes {
			if change.apiFileError != nil {
				fmt.Fprintf(stdout, "ERROR: package %s: could not read the package's .api file: %v\n", change.name, change.apiFileError)
				fmt.Fprintf(stdout, "ERROR: a readable .api file is required for all packages in project %s\n", change.projectName)
			} else {
				printChangeSummary(stdout, change, detailedOutput)
			}
		}
	}
	return nil
}

// cmdAPIUpdate represents the "v23 api fix" command.
var cmdAPIUpdate = &cmdline.Command{
	Runner:   cmdline.RunnerFunc(runAPIFix),
	Name:     "fix",
	Short:    "Update .api files to reflect changes to the public API",
	Long:     "Update .api files to reflect changes to the public API.",
	ArgsName: "<projects>",
	ArgsLong: "<projects> is a list of vanadium projects to update. If none are specified, all project APIs are updated.",
}

func runAPIFix(env *cmdline.Env, args []string) error {
	ctx := tool.NewContextFromEnv(env)
	config, err := util.LoadConfig(ctx)
	if err != nil {
		return err
	}
	changes, err := getPackageChanges(ctx, config.APICheckProjects(), args)
	if err != nil {
		return err
	}
	for _, change := range changes {
		if len(change.newAPIContent) == 0 {
			if _, err := ctx.Run().Stat(change.apiFilePath); !os.IsNotExist(err) {
				if err != nil {
					return err
				}
				// No API contents? Remove the file.
				if err := ctx.Run().RemoveAll(change.apiFilePath); err != nil {
					return err
				}
			}
		} else if err := ctx.Run().WriteFile(change.apiFilePath, []byte(change.newAPIContent), 0644); err != nil {
			return fmt.Errorf("WriteFile(%s) failed: %v", change.apiFilePath, err)
		}
		fmt.Fprintf(ctx.Stdout(), "Updated %s.\n", change.apiFilePath)
	}
	return nil
}

func main() {
	cmdline.Main(cmdAPI)
}