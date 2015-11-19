// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodejs

import (
	"flag"
	"fmt"
	"runtime"

	"v.io/jiri/profiles"
	"v.io/jiri/tool"
)

const (
	profileName = "nodejs"
)

type versionSpec struct {
	nodeVersion string
}

func init() {
	m := &Manager{
		versionInfo: profiles.NewVersionInfo(profileName, map[string]interface{}{
			"10.24": &versionSpec{"node-v0.10.24"},
		}, "10.24"),
	}
	profiles.Register(profileName, m)
}

type Manager struct {
	root, nodeRoot          profiles.RelativePath
	nodeSrcDir, nodeInstDir profiles.RelativePath
	versionInfo             *profiles.VersionInfo
	spec                    versionSpec
}

func (Manager) Name() string {
	return profileName
}

func (m Manager) String() string {
	return fmt.Sprintf("%s[%s]", profileName, m.versionInfo.Default())
}

func (m Manager) VersionInfo() *profiles.VersionInfo {
	return m.versionInfo
}

func (m Manager) Info() string {
	return `
The nodejs profile provides support for node. It installs and builds particular,
tested, versions of node.`
}

func (m *Manager) AddFlags(flags *flag.FlagSet, action profiles.Action) {}

func (m *Manager) initForTarget(root profiles.RelativePath, target profiles.Target) error {
	if err := m.versionInfo.Lookup(target.Version(), &m.spec); err != nil {
		return err
	}
	m.root = root
	m.nodeRoot = m.root.Join("cout", m.spec.nodeVersion)
	m.nodeInstDir = m.nodeRoot.Join(target.TargetSpecificDirname())
	m.nodeSrcDir = m.root.RootJoin("third_party", "csrc", m.spec.nodeVersion)
	return nil
}

func (m *Manager) Install(ctx *tool.Context, root profiles.RelativePath, target profiles.Target) error {
	if err := m.initForTarget(root, target); err != nil {
		return err
	}
	if target.CrossCompiling() {
		return fmt.Errorf("the %q profile does not support cross compilation to %v", profileName, target)
	}
	if err := m.installNode(ctx, target); err != nil {
		return err
	}
	if profiles.SchemaVersion() >= 4 {
		target.InstallationDir = m.nodeInstDir.RelativePath()
		profiles.InstallProfile(profileName, m.nodeRoot.RelativePath())
	} else {
		target.InstallationDir = m.nodeInstDir.Expand()
		profiles.InstallProfile(profileName, m.nodeRoot.Expand())
	}
	return profiles.AddProfileTarget(profileName, target)
}

func (m *Manager) Uninstall(ctx *tool.Context, root profiles.RelativePath, target profiles.Target) error {
	if err := m.initForTarget(root, target); err != nil {
		return err
	}
	if err := ctx.NewSeq().RemoveAll(m.nodeInstDir.Expand()).Done(); err != nil {
		return err
	}
	profiles.RemoveProfileTarget(profileName, target)
	return nil
}

func (m *Manager) installNode(ctx *tool.Context, target profiles.Target) error {
	switch target.OS() {
	case "darwin":
	case "linux":
		if err := profiles.InstallPackages(ctx, []string{"g++"}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%q is not supported", target.OS)
	}
	// Build and install NodeJS.
	installNodeFn := func() error {
		return ctx.NewSeq().Pushd(m.nodeSrcDir.Expand()).
			Run("./configure", fmt.Sprintf("--prefix=%v", m.nodeInstDir.Expand())).
			Run("make", "clean").
			Run("make", fmt.Sprintf("-j%d", runtime.NumCPU())).
			Last("make", "install")
	}
	return profiles.AtomicAction(ctx, installNodeFn, m.nodeInstDir.Expand(), "Build and install node.js")
}
