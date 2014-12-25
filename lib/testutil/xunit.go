package testutil

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"veyron.io/tools/lib/util"
)

type testSuites struct {
	Suites  []testSuite `xml:"testsuite"`
	XMLName xml.Name    `xml:"testsuites"`
}

type testSuite struct {
	Name     string     `xml:"name,attr"`
	Cases    []testCase `xml:"testcase"`
	Errors   int        `xml:"errors,attr"`
	Failures int        `xml:"failures,attr"`
	Skip     int        `xml:"skip,attr"`
	Tests    int        `xml:"tests,attr"`
}

type testCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Errors    []testError   `xml:"error"`
	Failures  []testFailure `xml:"failure"`
	Time      string        `xml:"time,attr"`
}

type testError struct {
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

type testFailure struct {
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

// xUnitReportPath returns the path to the xUnit file.
//
// TODO(jsimsa): Once all Jenkins shell test scripts are ported to Go,
// change the filename to xunit_report_<testName>.xml.
func XUnitReportPath(testName string) string {
	workspace, fileName := os.Getenv("WORKSPACE"), fmt.Sprintf("tests_%s.xml", strings.Replace(testName, "-", "_", -1))
	if workspace == "" {

		return filepath.Join(os.Getenv("HOME"), "tmp", testName, fileName)
	} else {
		return filepath.Join(workspace, fileName)
	}
}

// testSuiteFromGoTestOutput reads data from the given input, assuming
// it contains test results generated by "go test -v", and returns it
// as an in-memory data structure.
func testSuiteFromGoTestOutput(ctx *util.Context, testOutput io.Reader) (*testSuite, error) {
	root, err := util.VanadiumRoot()
	if err != nil {
		return nil, err
	}
	bin := filepath.Join(root, "environment", "golib", "bin", "go2xunit")
	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdin = testOutput
	opts.Stdout = &out
	if err := ctx.Run().CommandWithOpts(opts, bin); err != nil {
		return nil, err
	}
	var suite testSuite
	if err := xml.Unmarshal(out.Bytes(), &suite); err != nil {
		return nil, fmt.Errorf("Unmarshal() failed: %v\n%v", err, out.String())
	}
	return &suite, nil
}

// createXUnitReport generates an xUnit report using the given test
// suites.
func createXUnitReport(ctx *util.Context, testName string, suites []testSuite) error {
	result := testSuites{Suites: suites}
	bytes, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("MarshalIndent(%v) failed: %v", result, err)
	}
	if err := ctx.Run().WriteFile(XUnitReportPath(testName), bytes, os.FileMode(0644)); err != nil {
		return fmt.Errorf("WriteFile(%v) failed: %v", XUnitReportPath(testName), err)
	}
	return nil
}
