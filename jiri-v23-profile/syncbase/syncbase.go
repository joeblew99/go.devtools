// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"v.io/jiri/profiles"
	"v.io/jiri/tool"
	"v.io/x/lib/envvar"
)

const (
	profileName    = "syncbase"
	profileVersion = "1"
)

func init() {
	m := &Manager{
		versionInfo: profiles.NewVersionInfo(profileName, map[string]interface{}{
			"1": "1",
		}, "1"),
	}
	profiles.Register(profileName, m)
}

type Manager struct {
	syncbaseRoot, syncbaseInstRoot profiles.RelativePath
	snappySrcDir, leveldbSrcDir    profiles.RelativePath
	snappyInstDir, leveldbInstDir  profiles.RelativePath
	versionInfo                    *profiles.VersionInfo
}

func (Manager) Name() string {
	return profileName
}

func (m Manager) String() string {
	return fmt.Sprintf("%s[%s]", profileName, m.versionInfo.Default())
}

func (m Manager) Info() string {
	return `
The syncbase profile provides support for syncbase, in particular the snappy and
leveldb libraries.`
}

func (m Manager) VersionInfo() *profiles.VersionInfo {
	return m.versionInfo
}

func (m *Manager) AddFlags(flags *flag.FlagSet, action profiles.Action) {
}

func (m *Manager) initForTarget(ctx *tool.Context, root profiles.RelativePath, target profiles.Target) {
	m.syncbaseRoot = root.Join("cout")
	m.snappySrcDir = root.RootJoin("third_party", "csrc", "snappy-1.1.2")
	m.leveldbSrcDir = root.RootJoin("third_party", "csrc", "leveldb")

	targetDir := target.TargetSpecificDirname()
	m.syncbaseInstRoot = m.syncbaseRoot.Join(targetDir)
	m.snappyInstDir = m.syncbaseInstRoot.Join("snappy")
	m.leveldbInstDir = m.syncbaseInstRoot.Join("leveldb")

	if ctx.Verbose() {
		fmt.Fprintf(ctx.Stdout(), "Installation Directories for: %s\n", target)
		fmt.Fprintf(ctx.Stdout(), "Syncbase installation dir: %s\n", m.syncbaseInstRoot)
		fmt.Fprintf(ctx.Stdout(), "Snappy: %s\n", m.snappyInstDir)
		fmt.Fprintf(ctx.Stdout(), "Leveldb: %s\n", m.leveldbInstDir)
	}
}

func relPath(rp profiles.RelativePath) string {
	if profiles.SchemaVersion() >= 4 {
		return rp.String()
	}
	return rp.Expand()
}

// setSyncbaseEnv adds the LevelDB third-party C++ libraries Vanadium
// Go code depends on to the CGO_CFLAGS and CGO_LDFLAGS variables.
func (m *Manager) syncbaseEnv(ctx *tool.Context, target profiles.Target) ([]string, error) {
	env := envvar.VarsFromSlice([]string{})
	for _, dir := range []profiles.RelativePath{
		m.leveldbInstDir,
		m.snappyInstDir,
	} {
		cflags := env.GetTokens("CGO_CFLAGS", " ")
		cxxflags := env.GetTokens("CGO_CXXFLAGS", " ")
		ldflags := env.GetTokens("CGO_LDFLAGS", " ")
		if _, err := ctx.Run().Stat(dir.Expand()); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		cflags = append(cflags, filepath.Join("-I"+relPath(dir), "include"))
		cxxflags = append(cxxflags, filepath.Join("-I"+relPath(dir), "include"))
		ldflags = append(ldflags, filepath.Join("-L"+relPath(dir), "lib"))
		if target.Arch() == "linux" {
			ldflags = append(ldflags, "-Wl,-rpath", filepath.Join(relPath(dir), "lib"))
		}
		env.SetTokens("CGO_CFLAGS", cflags, " ")
		env.SetTokens("CGO_CXXFLAGS", cxxflags, " ")
		env.SetTokens("CGO_LDFLAGS", ldflags, " ")
	}
	return env.ToSlice(), nil
}

func (m *Manager) Install(ctx *tool.Context, root profiles.RelativePath, target profiles.Target) error {
	m.initForTarget(ctx, root, target)
	if err := m.installDependencies(ctx, target.Arch(), target.OS()); err != nil {
		return err
	}
	if err := m.installCommon(ctx, target); err != nil {
		return err
	}
	env := envvar.VarsFromSlice(target.Env.Vars)
	syncbaseEnv, err := m.syncbaseEnv(ctx, target)
	if err != nil {
		return err
	}
	profiles.MergeEnv(profiles.ProfileMergePolicies(), env, syncbaseEnv)
	target.Env.Vars = env.ToSlice()
	if profiles.SchemaVersion() >= 4 {
		target.InstallationDir = m.syncbaseInstRoot.RelativePath()
	} else {
		target.InstallationDir = m.syncbaseInstRoot.Expand()
	}

	profiles.InstallProfile(profileName, m.syncbaseRoot.RelativePath())
	return profiles.AddProfileTarget(profileName, target)
}

func (m *Manager) Uninstall(ctx *tool.Context, root profiles.RelativePath, target profiles.Target) error {
	m.initForTarget(ctx, root, target)
	if err := ctx.Run().RemoveAll(m.snappyInstDir.Expand()); err != nil {
		return err
	}
	if err := ctx.Run().RemoveAll(m.leveldbInstDir.Expand()); err != nil {
		return err
	}
	profiles.RemoveProfileTarget(profileName, target)
	return nil
}

func (m *Manager) installDependencies(ctx *tool.Context, arch, OS string) error {
	var pkgs []string
	switch runtime.GOOS {
	case "darwin":
		pkgs = []string{
			"autoconf", "automake", "libtool", "pkg-config",
		}
	case "linux":
		pkgs = []string{
			"autoconf", "automake", "g++", "g++-multilib", "gcc-multilib", "libtool", "pkg-config",
		}
	default:
		return fmt.Errorf("%q is not supported", runtime.GOOS)
	}
	return profiles.InstallPackages(ctx, pkgs)
}

func getAndroidRoot() (string, error) {
	androidProfile := profiles.LookupProfile("android")
	if androidProfile == nil {
		return "", fmt.Errorf("android profile is not installed")
	}
	return androidProfile.Root, nil
}

// installSyncbaseCommon installs the syncbase profile.
func (m *Manager) installCommon(ctx *tool.Context, target profiles.Target) (e error) {
	// Build and install Snappy.
	installSnappyFn := func() error {
		if err := ctx.Run().Chdir(m.snappySrcDir.Expand()); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "autoreconf", "--install", "--force", "--verbose"); err != nil {
			return err
		}
		args := []string{
			fmt.Sprintf("--prefix=%v", m.snappyInstDir.Expand()),
			"--enable-shared=false",
		}
		env := map[string]string{
			// NOTE(nlacasse): The -fPIC flag is needed to compile Syncbase Mojo service.
			"CXXFLAGS": " -fPIC",
		}
		if target.Arch() == "386" {
			env["CC"] = "gcc -m32"
			env["CXX"] = "g++ -m32"
		} else if target.Arch() == "arm" && target.OS() == "android" {
			androidRoot, err := getAndroidRoot()
			if err != nil {
				return err
			}
			args = append(args,
				"--host=arm-linux-androidabi",
				"--target=arm-linux-androidabi",
			)
			ndkRoot := filepath.Join(androidRoot, "ndk-toolchain")
			env["CC"] = filepath.Join(ndkRoot, "bin", "arm-linux-androideabi-gcc")
			env["CXX"] = filepath.Join(ndkRoot, "bin", "arm-linux-androideabi-g++")
			env["AR"] = filepath.Join(ndkRoot, "arm-linux-androideabi", "bin", "ar")
			env["RANLIB"] = filepath.Join(ndkRoot, "arm-linux-androideabi", "bin", "ranlib")
		} else if target.Arch() == "amd64" && runtime.GOOS == "linux" && target.OS() == "fnl" {
			root := os.Getenv("FNL_JIRI_ROOT")
			if len(root) == 0 {
				return fmt.Errorf("FNL_JIRI_ROOT not specified in the command line environment")
			}
			muslBin := filepath.Join(root, "out/root/tools/x86_64-fuchsia-linux-musl/bin")
			env["CC"] = filepath.Join(muslBin, "x86_64-fuchsia-linux-musl-gcc")
			env["CXX"] = filepath.Join(muslBin, "x86_64-fuchsia-linux-musl-g++")
			args = append(args, "--host=amd64-linux")
		} else if target.Arch() == "arm" && runtime.GOOS == "darwin" && target.OS() == "linux" {
			return fmt.Errorf("darwin -> arm-linux cross compilation not yet supported.")
			/*
			   export CC=/Volumes/code2/llvm/bin/cc-arm-raspian
			   export CXX=/Volumes/code2/llvm/bin/cxx-arm-raspian
			   export LDFLAGS=-lm
			   export AR=/Volumes/code2/llvm/install/binutils/bin/ar
			   export RANLIB=/Volumes/code2/llvm/install/binutils/bin/ranlib
			   ./configure --prefix=$(pwd)/../../cout/linux_arm/snappy --enable-shared=false \
			           --host=arm-linux-gnueabi
			*/
		}
		if err := profiles.RunCommand(ctx, env, "./configure", args...); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "make", "clean"); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "make", fmt.Sprintf("-j%d", runtime.NumCPU())); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "make", "install"); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "make", "distclean"); err != nil {
			return err
		}
		return nil
	}
	if err := profiles.AtomicAction(ctx, installSnappyFn, m.snappyInstDir.Expand(), "Build and install Snappy"); err != nil {
		return err
	}

	// Build and install LevelDB.
	installLeveldbFn := func() error {
		if err := ctx.Run().Chdir(m.leveldbSrcDir.Expand()); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, nil, "mkdir", "-p", m.leveldbInstDir.Expand()); err != nil {
			return err
		}
		leveldbIncludeDir := m.leveldbInstDir.Join("include").Expand()
		if err := profiles.RunCommand(ctx, nil, "cp", "-R", "include", leveldbIncludeDir); err != nil {
			return err
		}
		leveldbLibDir := m.leveldbInstDir.Join("lib").Expand()
		if err := profiles.RunCommand(ctx, nil, "mkdir", leveldbLibDir); err != nil {
			return err
		}
		env := map[string]string{
			"PREFIX": leveldbLibDir,
			// NOTE(nlacasse): The -fPIC flag is needed to compile Syncbase Mojo service.
			"CXXFLAGS": "-I" + filepath.Join(relPath(m.snappyInstDir), "include") + " -fPIC",
			"LDFLAGS":  "-L" + filepath.Join(relPath(m.snappyInstDir), "lib"),
		}
		if target.Arch() == "386" {
			env["CC"] = "gcc -m32"
			env["CXX"] = "g++ -m32"
		} else if target.Arch() == "arm" && target.OS() == "android" {
			androidRoot, err := getAndroidRoot()
			if err != nil {
				return err
			}
			ndkRoot := filepath.Join(androidRoot, "ndk-toolchain")
			env["CC"] = filepath.Join(ndkRoot, "bin", "arm-linux-androideabi-gcc")
			env["CXX"] = filepath.Join(ndkRoot, "bin", "arm-linux-androideabi-g++")
			env["TARGET_OS"] = "OS_ANDROID_CROSSCOMPILE"
			env["AR"] = filepath.Join(ndkRoot, "arm-linux-androideabi", "bin", "ar")
			env["RANLIB"] = filepath.Join(ndkRoot, "arm-linux-androideabi", "bin", "ranlib")
		} else if target.Arch() == "amd64" && runtime.GOOS == "linux" && target.OS() == "fnl" {
			root := os.Getenv("FNL_JIRI_ROOT")
			if len(root) == 0 {
				return fmt.Errorf("FNL_JIRI_ROOT not specified in the command line environment")
			}
			muslBin := filepath.Join(root, "out/root/tools/x86_64-fuchsia-linux-musl/bin")
			env["CC"] = filepath.Join(muslBin, "x86_64-fuchsia-linux-musl-gcc")
			env["CXX"] = filepath.Join(muslBin, "x86_64-fuchsia-linux-musl-g++")
			env["AR"] = filepath.Join(muslBin, "x86_64-fuchsia-linux-musl-ar")
		} else if target.Arch() == "arm" && runtime.GOOS == "darwin" && target.OS() == "linux" {
			return fmt.Errorf("darwin -> arm-linux cross compilation not yet supported.")
			/*
				export CC=/Volumes/code2/llvm/bin/cc-arm-raspian
				export CXX=/Volumes/code2/llvm/bin/cxx-arm-raspian
				export TARGET_OS=Linux
				export AR=/Volumes/code2/llvm/install/binutils/bin/ar
				export RANLIB=/Volumes/code2/llvm/install/binutils/bin/ranlib
				INST_DIR=../../cout/linux_arm/leveldb
				mkdir -p $INST_DIR
				mkdir -p $INST_DIR/lib
				mkdir -p $INST_DIR/include
				export PREFIX=../../cout/linux_arm/leveldb/lib
				make static
				cp -r ./include/leveldb ../../cout/linux_arm/leveldb/include
			*/
		}
		if err := profiles.RunCommand(ctx, env, "make", "clean"); err != nil {
			return err
		}
		if err := profiles.RunCommand(ctx, env, "make", "static"); err != nil {
			return err
		}
		return nil
	}
	if err := profiles.AtomicAction(ctx, installLeveldbFn, m.leveldbInstDir.Expand(), "Build and install LevelDB"); err != nil {
		return err
	}
	return nil
}
