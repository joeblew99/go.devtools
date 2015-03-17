package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/x/devtools/internal/tool"
	"v.io/x/devtools/internal/util"
	"v.io/x/lib/cmdline"
)

func createLabelDir(t *testing.T, ctx *tool.Context, snapshotDir, name string, snapshots []string) {
	labelDir, perm := filepath.Join(snapshotDir, "labels", name), os.FileMode(0700)
	if err := ctx.Run().MkdirAll(labelDir, perm); err != nil {
		t.Fatalf("MkdirAll(%v, %v) failed: %v", labelDir, perm, err)
	}
	for i, snapshot := range snapshots {
		path := filepath.Join(labelDir, snapshot)
		_, err := os.Create(path)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if i == 0 {
			symlinkPath := filepath.Join(snapshotDir, name)
			if err := ctx.Run().Symlink(path, symlinkPath); err != nil {
				t.Fatalf("Symlink(%v, %v) failed: %v", path, symlinkPath, err)
			}
		}
	}
}

func generateOutput(labels []label) string {
	output := ""
	for _, label := range labels {
		output += fmt.Sprintf("snapshots of label %q:\n", label.name)
		for _, snapshot := range label.snapshots {
			output += fmt.Sprintf("  %v\n", snapshot)
		}
	}
	return output
}

type config struct {
	remote bool
	dir    string
}

type label struct {
	name      string
	snapshots []string
}

func TestList(t *testing.T) {
	ctx := tool.NewDefaultContext()

	// Setup a fake VANADIUM_ROOT.
	root, err := util.NewFakeVanadiumRoot(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer func() {
		if err := root.Cleanup(ctx); err != nil {
			t.Fatalf("%v", err)
		}
	}()
	oldRoot, err := util.VanadiumRoot()
	if err := os.Setenv("VANADIUM_ROOT", root.Dir); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("VANADIUM_ROOT", oldRoot)

	manifestDir, err := util.ManifestDir()
	if err != nil {
		t.Fatalf("%v", err)
	}
	snapshotDir, err := util.LocalSnapshotDir()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Create a test suite.
	tests := []config{
		config{
			remote: false,
			dir:    snapshotDir,
		},
		config{
			remote: true,
			dir:    manifestDir,
		},
	}
	labels := []label{
		label{
			name:      "beta",
			snapshots: []string{"beta-1", "beta-2", "beta-3"},
		},
		label{
			name:      "stable",
			snapshots: []string{"stable-1", "stable-2", "stable-3"},
		},
	}

	for _, test := range tests {
		remoteFlag = test.remote
		// Create the snapshots directory and populate it with the
		// data specified by the test suite.
		for _, label := range labels {
			createLabelDir(t, ctx, test.dir, label.name, label.snapshots)
		}

		// Check that running "v23 snapshot list" with no arguments
		// returns the expected output.
		var stdout bytes.Buffer
		command := cmdline.Command{}
		command.Init(nil, &stdout, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := runSnapshotList(&command, nil); err != nil {
			t.Fatalf("%v", err)
		}
		got, want := stdout.String(), generateOutput(labels)
		if got != want {
			t.Fatalf("unexpected output:\ngot\n%v\nwant\n%v\n", got, want)
		}

		// Check that running "v23 snapshot list" with one argument
		// returns the expected output.
		stdout.Reset()
		if err := runSnapshotList(&command, []string{"stable"}); err != nil {
			t.Fatalf("%v", err)
		}
		got, want = stdout.String(), generateOutput(labels[1:])
		if got != want {
			t.Fatalf("unexpected output:\ngot\n%v\nwant\n%v\n", got, want)
		}

		// Check that running "v23 snapshot list" with
		// multiple arguments returns the expected output.
		stdout.Reset()
		if err := runSnapshotList(&command, []string{"beta", "stable"}); err != nil {
			t.Fatalf("%v", err)
		}
		got, want = stdout.String(), generateOutput(labels)
		if got != want {
			t.Fatalf("unexpected output:\ngot\n%v\nwant\n%v\n", got, want)
		}
	}
}

func checkReadme(t *testing.T, ctx *tool.Context, project, message string) {
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("%v", err)
	}
	readmeFile := filepath.Join(project, "README")
	data, err := ioutil.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("ReadFile(%v) failed: %v", readmeFile, err)
	}
	if got, want := data, []byte(message); bytes.Compare(got, want) != 0 {
		t.Fatalf("unexpected content %v:\ngot\n%s\nwant\n%s\n", project, got, want)
	}
}

func localProjectName(i int) string {
	return "test-local-project-" + fmt.Sprintf("%d", i+1)
}

func remoteProjectName(i int) string {
	return "test-remote-project-" + fmt.Sprintf("%d", i+1)
}

func writeReadme(t *testing.T, ctx *tool.Context, projectDir, message string) {
	path, perm := filepath.Join(projectDir, "README"), os.FileMode(0644)
	if err := ctx.Run().WriteFile(path, []byte(message), perm); err != nil {
		t.Fatalf("%v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer ctx.Run().Chdir(cwd)
	if err := ctx.Run().Chdir(projectDir); err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Git().CommitFile(path, "creating README"); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestCreate(t *testing.T) {
	// Setup a fake VANADIUM_ROOT instance.
	ctx := tool.NewDefaultContext()
	root, err := util.NewFakeVanadiumRoot(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer func() {
		if err := root.Cleanup(ctx); err != nil {
			t.Fatalf("%v", err)
		}
	}()

	// Setup the initial remote and local projects.
	numProjects, remoteProjects := 2, []string{}
	for i := 0; i < numProjects; i++ {
		root.CreateRemoteProject(ctx, remoteProjectName(i))
		root.AddProject(ctx, util.Project{
			Name:   remoteProjectName(i),
			Path:   localProjectName(i),
			Remote: root.Projects[remoteProjectName(i)],
		})
	}

	// Create a fake configuration file.
	config := util.NewConfig(util.SnapshotLabelTestsOpt(map[string][]string{"remote-snapshot": []string{}}))
	root.WriteLocalToolsConfig(ctx, config)

	oldRoot, err := util.VanadiumRoot()
	if err := os.Setenv("VANADIUM_ROOT", root.Dir); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("VANADIUM_ROOT", oldRoot)

	// Create initial commits in the remote projects and use
	// UpdateUniverse() to mirror them locally.
	for i := 0; i < numProjects; i++ {
		writeReadme(t, ctx, root.Projects[remoteProjectName(i)], "revision 1")
	}
	if err := util.UpdateUniverse(ctx, true); err != nil {
		t.Fatalf("%v", err)
	}

	// Create a local snapshot.
	command := cmdline.Command{}
	command.Init(nil, nil, nil)
	remoteFlag = false
	if err := runSnapshotCreate(&command, []string{"local-snapshot"}); err != nil {
		t.Fatalf("%v", err)
	}

	// Remove the local project repositories.
	for i, _ := range remoteProjects {
		localProject := filepath.Join(root.Dir, localProjectName(i))
		if err := ctx.Run().RemoveAll(localProject); err != nil {
			t.Fatalf("%v", err)
		}
	}

	// Check that invoking the UpdateUniverse() with the local
	// snapshot restores the local repositories.
	snapshotDir, err := util.LocalSnapshotDir()
	if err != nil {
		t.Fatalf("%v", err)
	}
	snapshotFile := filepath.Join(snapshotDir, "local-snapshot")
	localCtx := ctx.Clone(tool.ContextOpts{
		Manifest: &snapshotFile,
	})
	if err := util.UpdateUniverse(localCtx, true); err != nil {
		t.Fatalf("%v", err)
	}
	for i, _ := range remoteProjects {
		localProject := filepath.Join(root.Dir, localProjectName(i))
		checkReadme(t, ctx, localProject, "revision 1")
	}

	// Create a remote snapshot.
	remoteFlag = true
	manifest := "remote-snapshot"
	root.EnableRemoteManifestPush(ctx)
	if err := runSnapshotCreate(&command, []string{manifest}); err != nil {
		t.Fatalf("%v", err)
	}

	// Remove the local project repositories.
	for i, _ := range remoteProjects {
		localProject := filepath.Join(root.Dir, localProjectName(i))
		if err := ctx.Run().RemoveAll(localProject); err != nil {
			t.Fatalf("%v", err)
		}
	}

	// Check that invoking the UpdateUniverse() with the remote snapshot.
	remoteCtx := ctx.Clone(tool.ContextOpts{
		Manifest: &manifest,
	})
	if err := util.UpdateUniverse(remoteCtx, true); err != nil {
		t.Fatalf("%v", err)
	}
	for i, _ := range remoteProjects {
		localProject := filepath.Join(root.Dir, localProjectName(i))
		checkReadme(t, ctx, localProject, "revision 1")
	}
}
