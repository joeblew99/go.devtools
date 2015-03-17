package main

import (
	"os"
	"reflect"
	"testing"

	"v.io/x/devtools/internal/tool"
	"v.io/x/devtools/internal/util"
)

func TestJenkinsTestsToStart(t *testing.T) {
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
	config := util.NewConfig(
		util.ProjectTestsOpt(map[string][]string{
			"release.go.core": []string{"go", "javascript"},
			"release.js.core": []string{"javascript"},
		}),
		util.TestGroupsOpt(map[string][]string{
			"go":         []string{"vanadium-go-build", "vanadium-go-test", "vanadium-go-race"},
			"javascript": []string{"vanadium-js-integration", "vanadium-js-unit"},
		}),
	)
	if err := root.WriteLocalToolsConfig(ctx, config); err != nil {
		t.Fatalf("%v", err)
	}

	oldRoot, err := util.VanadiumRoot()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := os.Setenv("VANADIUM_ROOT", root.Dir); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("VANADIUM_ROOT", oldRoot)

	testCases := []struct {
		projects            []string
		expectedJenkinsTest []string
	}{
		{
			projects: []string{"release.go.core"},
			expectedJenkinsTest: []string{
				"vanadium-go-build",
			},
		},
		{
			projects: []string{"release.js.core"},
			expectedJenkinsTest: []string{
				"vanadium-js-unit",
			},
		},
	}

	for _, test := range testCases {
		got, err := jenkinsTestsToStart(ctx, test.projects)
		if err != nil {
			t.Fatalf("want no errors, got: %v", err)
		}
		if !reflect.DeepEqual(test.expectedJenkinsTest, got) {
			t.Fatalf("want %v, got %v", test.expectedJenkinsTest, got)
		}
	}
}
