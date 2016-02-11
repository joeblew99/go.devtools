// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
bendroid attempts to emulate the behavior of go test, but running the tests and
benchmarks on an android device.

Note that currently we support only a small subset of the flags allowed to 'go
test'.

We depend on gradle and adb, so those tools should be in your path.

You should also set relevant CGO environment variables (for example pointing at
the ndk gcc and g++) see: https://golang.org/cmd/cgo/.  Unlike gomobile, we
don't set them for you. In particular, CC and CXX must be set to point to the
binaries from the Android NDK toolchain. For example, if the toolchain is
installed in $NDK_TOOLCHAIN, then:
  export CC=${NDK_TOOLCHAIN}/arm-21/bin/arm-linux-androideabi-gcc
  export CXX=${NDK_TOOLCHAIN}/arm-21/bin/arm-linux-androideabi-g++
before running bendroid.

Finally, bendroid requires Go 1.6 or above (since that contains some changes to
make the generated shared library compatible with Android SDK version 23).

Usage:
   bendroid [flags] [-c] [build and test flags] [packages] [flags for test binary]

The global flags are:
 -bench=
   Run benchmarks matching the regular expression. By default, no benchmarks
   run. To run all benchmarks, use '-bench .' or '-bench=.'.
 -benchmem=false
   Print memory allocation statistics for benchmarks.
 -benchtime=1s
   Print memory allocation statistics for benchmarks.
 -c=false
   Compile the test binary to pkg.test but do not run it (where pkg is the last
   element of the package's import path). The file name can be changed with the
   -o flag.
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -o=
   Compile the test binary to the named file. The test still runs (unless -c is
   specified).
 -run=
   Run only those tests and examples matching the regular expression.
 -tags=
   a list of build tags to consider satisfied during the build. For more
   information about build tags, see the description of build constraints in the
   documentation for the go/build package.
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=false
   Verbose output: log all tests as they are run. Also print all text from Log
   and Logf calls even if the test succeeds.
 -work=false
   print the name of the temporary work directory and do not delete it when
   exiting.
*/
package main
