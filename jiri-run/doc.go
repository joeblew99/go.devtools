// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Run an executable using the specified profile and target's environment.

Usage:
   jiri run [flags] <executable> [arg ...]

<executable> [arg ...] is the executable to run and any arguments to pass
verbatim to the executable.

The jiri run flags are:
 -color=true
   Use color to format output.
 -merge-policies=+CCFLAGS,+CGO_CFLAGS,+CGO_CXXFLAGS,+CGO_LDFLAGS,+CXXFLAGS,GOARCH,GOOS,GOPATH:,^GOROOT*,+LDFLAGS,:PATH,VDLPATH:
   specify policies for merging environment variables
 -n=false
   Show what commands will run but do not execute them.
 -profiles=base,jiri
   a comma separated list of profiles to use
 -profiles-manifest=$JIRI_ROOT/.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -skip-profiles=false
   if set, no profiles will be used
 -target=<runtime.GOARCH>-<runtime.GOOS>
   specifies a profile target in the following form:
   <arch>-<os>[@<version>]|<arch>-<val>[@<version>]
 -v=false
   Print verbose output.

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=false
   print verbose debugging information
*/
package main
