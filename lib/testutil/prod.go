package testutil

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"v.io/tools/lib/collect"
	"v.io/tools/lib/util"
)

var (
	signatureRE = regexp.MustCompile(`^func (.*)\(.*\) \(.*\)$`)
)

// methods parses the given signature, which is expected to be
// generated by the "vrpc describe ..." command, extracting the list
// of methods contained in the signature of a vanadium RPC server the
// input describes.
func methods(signature string) ([]string, error) {
	signature = strings.TrimSpace(signature)
	result := []string{}
	lines := strings.Split(signature, "\n")
	for _, line := range lines {
		if !signatureRE.MatchString(line) {
			return nil, fmt.Errorf("unexpected line in service signature: %v", line)
		}
		matches := signatureRE.FindStringSubmatch(line)
		if len(matches) != 2 {
			return nil, fmt.Errorf("unexpected line in services signature: %v", line)
		}
		result = append(result, matches[1])
	}
	sort.Strings(result)
	return result, nil
}

// generateTestSuite generates an xUnit test suite that encapsulates
// the given input.
func generateTestSuite(ctx *util.Context, success bool, pkg string, duration time.Duration, output string) *testSuite {
	// Generate an xUnit test suite describing the result.
	s := testSuite{Name: pkg}
	c := testCase{
		Classname: pkg,
		Name:      "Test",
		Time:      fmt.Sprintf("%.2f", duration.Seconds()),
	}
	if !success {
		fmt.Fprintf(ctx.Stdout(), "%s ... failed\n%v\n", pkg, output)
		f := testFailure{
			Message: "vrpc",
			Data:    output,
		}
		c.Failures = append(c.Failures, f)
		s.Failures++
	} else {
		fmt.Fprintf(ctx.Stdout(), "%s ... ok\n", pkg)
	}
	s.Tests++
	s.Cases = append(s.Cases, c)
	return &s
}

// testProdService test the given production service.
func testProdService(ctx *util.Context, service prodService) (*testSuite, error) {
	root, err := util.VanadiumRoot()
	if err != nil {
		return nil, err
	}
	bin := filepath.Join(root, "release", "go", "bin", "vrpc")
	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdout = &out
	opts.Stderr = &out
	start := time.Now()
	if err := ctx.Run().TimedCommandWithOpts(DefaultTestTimeout, opts, bin, "describe", service.objectName); err != nil {
		return generateTestSuite(ctx, false, service.name, time.Now().Sub(start), out.String()), nil
	}
	output := out.String()
	got, err := methods(output)
	if err != nil {
		return generateTestSuite(ctx, false, service.name, time.Now().Sub(start), err.Error()), nil
	}
	if want := service.signature; !reflect.DeepEqual(got, want) {
		fmt.Fprintf(ctx.Stderr(), "mismatching methods: got %v, want %v\n", got, want)
		return generateTestSuite(ctx, false, service.name, time.Now().Sub(start), "mismatching signature"), nil
	}
	return generateTestSuite(ctx, true, service.name, time.Now().Sub(start), ""), nil
}

type prodService struct {
	name       string
	objectName string
	signature  []string
}

// vanadiumProdServicesTest runs a test of vanadium production services.
func vanadiumProdServicesTest(ctx *util.Context, testName string) (_ *TestResult, e error) {
	// Initialize the test.
	cleanup, err := initTest(ctx, testName, nil)
	if err != nil {
		return nil, err
	}
	defer collect.Error(func() error { return cleanup() }, &e)

	// Install the vrpc tool.
	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stderr = io.MultiWriter(&out, opts.Stderr)
	if err := ctx.Run().CommandWithOpts(opts, "v23", "go", "install", "v.io/core/veyron/tools/vrpc"); err != nil {
		// TODO(jingjin): create a utility function for this logic. See more in javascript.go.
		s := createTestSuiteWithFailure(testName, "BuildTools", "build failure", out.String(), 0)
		if err := createXUnitReport(ctx, testName, []testSuite{*s}); err != nil {
			return nil, err
		}
		return &TestResult{Status: TestFailed}, nil
	}

	// Describe the test cases.
	namespaceRoot := "/ns.dev.v.io:8101"
	allPassed, suites := true, []testSuite{}
	services := []prodService{
		prodService{
			name:       "mounttable",
			objectName: namespaceRoot,
			signature:  []string{"Delete", "GetACL", "Mount", "ResolveStep", "ResolveStepX", "SetACL", "Unmount"},
		},
		/* TODO(ashankar): Restore after use of ACLs and access to the Signature method has been resolved
		prodService{
			name:       "application repository",
			objectName: namespaceRoot + "/applicationd",
			signature:  []string{"Match", "Put", "Remove"},
		},
		prodService{
			name:       "binary repository",
			objectName: namespaceRoot + "/binaryd",
			signature:  []string{"Create", "Delete", "Download", "DownloadURL", "Stat", "Upload"},
		},
		*/
		prodService{
			name:       "macaroon service",
			objectName: namespaceRoot + "/identity/dev.v.io/macaroon",
			signature:  []string{"Bless"},
		},
		prodService{
			name:       "google identity service",
			objectName: namespaceRoot + "/identity/dev.v.io/google",
			signature:  []string{"BlessUsingAccessToken"},
		},
		prodService{
			name:       "binary discharger",
			objectName: namespaceRoot + "/identity/dev.v.io/discharger",
			signature:  []string{"Discharge"},
		},
	}

	for _, service := range services {
		suite, err := testProdService(ctx, service)
		if err != nil {
			return nil, err
		}
		allPassed = allPassed && (suite.Failures == 0)
		suites = append(suites, *suite)
	}

	// Create the xUnit report.
	if err := createXUnitReport(ctx, testName, suites); err != nil {
		return nil, err
	}
	if !allPassed {
		return &TestResult{Status: TestFailed}, nil
	}
	return &TestResult{Status: TestPassed}, nil
}
