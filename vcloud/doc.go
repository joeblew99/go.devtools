// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
The vcloud tool is a wrapper over the gcloud GCE resource management tool, to
simplify common usage scenarios.

Usage:
   vcloud <command>

The vcloud commands are:
   list        List GCE node information
   cp          Copy files to/from GCE node(s)
   sh          Start a shell or run a command on GCE node(s)
   run         Copy file(s) to GCE node(s) and run
   help        Display help for commands or topics
Run "vcloud help [command]" for command usage.

The global flags are:
 -color=false
   Format output in color.
 -n=false
   Show what commands will run, but do not execute them.
 -project=google.com:veyron
   Specify the gcloud project.
 -user=veyron
   Run operations as the given user on each node.
 -v=false
   Print verbose output.

Vcloud List

List GCE node information.  Runs 'gcloud compute instances list'.

Usage:
   vcloud list [flags] [nodes]

[nodes] is a comma-separated list of node name(s).  Each node name is a regular
expression, with matches performed on the full node name.  We select nodes that
match any of the regexps.  The comma-separated list allows you to easily specify
a list of specific node names, without using regexp alternation.  We assume node
names do not have embedded commas.

If [nodes] is not provided, lists information for all nodes.

The vcloud list flags are:
 -noheader=false
   Don't print list table header.

Vcloud Cp

Copy files to GCE node(s).  Runs 'gcloud compute copy-files'.  The default is to
copy to/from all nodes in parallel.

Usage:
   vcloud cp [flags] <nodes> <src...> <dst>

<nodes> is a comma-separated list of node name(s).  Each node name is a regular
expression, with matches performed on the full node name.  We select nodes that
match any of the regexps.  The comma-separated list allows you to easily specify
a list of specific node names, without using regexp alternation.  We assume node
names do not have embedded commas.

<src...> are the source file argument(s) to 'gcloud compute copy-files', and
<dst> is the destination.  The syntax for each file is:
  [:]file

Files with the ':' prefix are remote; files without any such prefix are local.

As with 'gcloud compute copy-files', if <dst> is local, all <src...> must be
remote.  If <dst> is remote, all <src...> must be local.

Each matching node in <nodes> is applied to the remote side of the copy
operation, either src or dst.  If <dst> is local and there is more than one
matching node, sub directories will be automatically created under <dst>.

E.g. if <nodes> matches A, B and C:
  // Copies local src{1,2,3} to {A,B,C}:dst
  vcloud cp src1 src2 src3 :dst
  // Copies remote {A,B,C}:src{1,2,3} to dst/{A,B,C} respectively.
  vcloud cp :src1 :src2 :src3 dst

The vcloud cp flags are:
 -failfast=false
   Skip unstarted nodes after the first failing node.
 -p=-1
   Copy to/from this many nodes in parallel.
     <0   means all nodes in parallel
      0,1 means sequentially
      2+  means at most this many nodes in parallel

Vcloud Sh

Start a shell or run a command on GCE node(s).  Runs 'gcloud compute ssh'.

Usage:
   vcloud sh [flags] <nodes> [command...]

<nodes> is a comma-separated list of node name(s).  Each node name is a regular
expression, with matches performed on the full node name.  We select nodes that
match any of the regexps.  The comma-separated list allows you to easily specify
a list of specific node names, without using regexp alternation.  We assume node
names do not have embedded commas.

[command...] is the shell command line to run on each node.  Specify the entire
command line without extra quoting, e.g. like this:
  vcloud sh jenkins-node uname -a
But NOT like this:
  vcloud sh jenkins-node 'uname -a'
If quoting and escaping becomes too complicated, use 'vcloud run' instead.

If <nodes> matches exactly one node and no [command] is given, sh starts a shell
on the specified node.

Otherwise [command...] is required; sh runs the command on all matching nodes.
The default is to run on all nodes in parallel.

The vcloud sh flags are:
 -failfast=false
   Skip unstarted nodes after the first failing node.
 -p=-1
   Run command on this many nodes in parallel.
     <0   means all nodes in parallel
      0,1 means sequentially
      2+  means at most this many nodes in parallel

Vcloud Run

Copy file(s) to GCE node(s) and run.  Uses the logic of both cp and sh.

Usage:
   vcloud run [flags] <nodes> <files...> [++ [command...]]

<nodes> is a comma-separated list of node name(s).  Each node name is a regular
expression, with matches performed on the full node name.  We select nodes that
match any of the regexps.  The comma-separated list allows you to easily specify
a list of specific node names, without using regexp alternation.  We assume node
names do not have embedded commas.

<files...> are the local source file argument(s) to copy to each matching node.

[command...] is the shell command line to run on each node.  Specify the entire
command line without extra quoting, just like 'vcloud sh'.  If a command is
specified, it must be preceeded by a single ++ argument, to distinguish it from
the files.  If no command is given, runs the first file from <files...>.

We run the following logic on each matching node, in parallel by default:
  1) Create a temporary directory based on a random number.
  2) Copy all files into the temporary directory.
  3) Runs the [command...], or if no command is given, runs the first run file.
     All occurrences of the string literal '+TMPDIR' are replaced in the command
     with the temporary directory.  No replacement occurs for the run files,
     since the run files are all local.
  4) Delete the temporary directory.

The vcloud run flags are:
 -failfast=false
   Skip unstarted nodes after the first failing node.
 -p=-1
   Copy/run on this many nodes in parallel.
     <0   means all nodes in parallel
      0,1 means sequentially
      2+  means at most this many nodes in parallel

Vcloud Help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

The output is formatted to a target width in runes.  The target width is
determined by checking the environment variable CMDLINE_WIDTH, falling back on
the terminal width from the OS, falling back on 80 chars.  By setting
CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0 the width is unlimited, and
if x == 0 or is unset one of the fallbacks is used.

Usage:
   vcloud help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vcloud help flags are:
 -style=text
   The formatting style for help output, either "text" or "godoc".
*/
package main