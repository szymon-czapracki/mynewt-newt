/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package porcelain describes a project in a format meant for scripts rather
// than people: "newt info --porcelain".  The format is stable; fields may be
// added, but existing ones are not renamed or removed without increasing
// FormatVersion.
package porcelain

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cast"

	"mynewt.apache.org/newt/newt/builder"
	"mynewt.apache.org/newt/newt/downloader"
	"mynewt.apache.org/newt/newt/newtutil"
	"mynewt.apache.org/newt/newt/pkg"
	"mynewt.apache.org/newt/newt/project"
	"mynewt.apache.org/newt/newt/repo"
	"mynewt.apache.org/newt/newt/syscfg"
	"mynewt.apache.org/newt/newt/target"
	"mynewt.apache.org/newt/util"
)

// FormatVersion is increased whenever a change to the output can break
// existing readers.
const FormatVersion = 1

// Info is everything the output describes.  Paths are relative to the
// project's directory, except for the project's own path.
type Info struct {
	FormatVersion int
	NewtVersion   string
	Project       Project
	Repos         []Repo
	Targets       []Target
	Packages      []Package
}

type Project struct {
	Name string
	Path string
}

type Repo struct {
	Name string
	Path string
	// Whether this is the project itself rather than a dependency.
	Local bool
	Url   string
	// The version project.yml asks for; empty if the repo is only a
	// dependency of another repo.
	Requested string
	// The installed version, and its commit; empty if not installed.
	Installed string
	Commit    string
	// What is modified in the repo's working tree; empty if clean.
	Dirty string
	// Only known when remote repos were queried.
	NeedsUpgrade bool
	// Why the repo's state could not be read, if it could not.
	Error string
}

type Target struct {
	Name string
	Repo string
	Path string
	// The target's variables without the "target." prefix, e.g. "app" and
	// "bsp", and its "cflags", "cxxflags", "lflags" and "aflags".  Lists
	// are joined with spaces; empty variables are left out.
	Vars map[string]string
	// All syscfg settings of the target's build, sorted by name, and the
	// problems that would make "newt build" fail.
	Syscfg       []Setting
	SyscfgErrors []string
}

// Setting is one resolved syscfg setting, as "newt target config show"
// displays it.
type Setting struct {
	Name  string
	Value string
	// The value from the setting's definition.
	Default     string
	Description string
	// "raw", "task_priority" or "flash_owner".
	Type string
	// "good", "const", "deprecated", "defunct" or "experimental".
	State string
	// The package that defines the setting; "newt" for settings newt
	// injects.
	Package string
	// The setting the value is copied from, if any.
	ValueRef     string
	Choices      []string
	Restrictions []Restriction
	// Where the value comes from, oldest (the definition) first.
	History []SettingPoint
}

type Restriction struct {
	// "notnull", "expr" or "range".
	Type string
	// The expression, or the ranges, e.g. "1..10, 16".
	Expr string
}

type SettingPoint struct {
	// The package that sets the value; "newt" for values newt sets.
	Source string
	Value  string
}

type Package struct {
	Name        string
	Type        string
	Repo        string
	Path        string
	Description string
}

// Gather collects the information about a project.  With remote, the repos'
// remotes are queried to tell whether they can be upgraded.
func Gather(proj *project.Project, remote bool) (*Info, error) {
	info := &Info{
		FormatVersion: FormatVersion,
		NewtVersion:   newtutil.NewtVersionStr,
		Project: Project{
			Name: proj.Name(),
			Path: proj.Path(),
		},
		Repos:    []Repo{},
		Targets:  []Target{},
		Packages: []Package{},
	}

	repoInfos, err := proj.RepoInfos(
		func(r *repo.Repo) bool { return true }, remote)
	if err != nil {
		return nil, err
	}

	for _, r := range proj.SelectRepos(func(r *repo.Repo) bool { return true }) {
		ri := repoInfos[r.Name()]
		if r.IsLocal() && ri.CommitHash == "" {
			// The project is not a git repo, which is not an error.
			ri.ErrorText = ""
		}
		pr := Repo{
			Name:         r.Name(),
			Path:         relPath(proj, r.Path()),
			Local:        r.IsLocal(),
			Url:          repoUrl(r),
			Commit:       ri.CommitHash,
			Dirty:        ri.DirtyState,
			NeedsUpgrade: ri.NeedsUpgrade,
			Error:        ri.ErrorText,
		}
		if req := proj.RepoRequirement(r.Name()); req != nil {
			pr.Requested = req.String()
		}
		if ri.InstalledVer != nil {
			pr.Installed = ri.InstalledVer.String()
		}
		info.Repos = append(info.Repos, pr)
	}

	for name, t := range target.GetTargets() {
		// The unittest target is used internally by newt.
		if strings.HasSuffix(name, "/unittest") {
			continue
		}

		pt := Target{
			Name: name,
			Repo: t.Package().Repo().Name(),
			Path: relPath(proj, t.Package().BasePath()),
			Vars: targetVars(t),
		}
		pt.Syscfg, pt.SyscfgErrors = targetSyscfg(t)
		info.Targets = append(info.Targets, pt)
	}
	sort.Slice(info.Targets, func(i, j int) bool {
		return info.Targets[i].Name < info.Targets[j].Name
	})

	for _, pkgs := range proj.PackageList() {
		for _, p := range *pkgs {
			lpkg, ok := p.(*pkg.LocalPackage)
			if !ok {
				continue
			}

			pp := Package{
				Name: lpkg.FullName(),
				Type: pkg.PackageTypeNames[lpkg.Type()],
				Repo: lpkg.Repo().Name(),
				Path: relPath(proj, lpkg.BasePath()),
			}
			if desc := lpkg.Desc(); desc != nil {
				pp.Description = strings.TrimSpace(desc.Description)
			}
			info.Packages = append(info.Packages, pp)
		}
	}
	sort.Slice(info.Packages, func(i, j int) bool {
		return info.Packages[i].Name < info.Packages[j].Name
	})

	return info, nil
}

func targetVars(t *target.Target) map[string]string {
	vars := map[string]string{}
	for k, v := range t.TargetY.AllSettings() {
		vars[strings.TrimPrefix(k, "target.")] = valueString(v)
	}

	// The build flags come from the target's package rather than target.yml.
	for _, name := range []string{"cflags", "cxxflags", "lflags", "aflags"} {
		flags, err := t.Package().PkgY.GetValStringSlice("pkg."+name, nil)
		util.OneTimeWarningError(err)
		vars[name] = strings.Join(flags, " ")
	}

	for k, v := range vars {
		if v == "" {
			delete(vars, k)
		}
	}

	return vars
}

// targetSyscfg resolves the target's configuration the way "newt target
// config show" does.  A target that cannot be resolved, e.g. because it has
// no app, has no settings and the reason as its only error.
func targetSyscfg(t *target.Target) ([]Setting, []string) {
	settings := []Setting{}

	b, err := builder.NewTargetBuilder(t)
	if err != nil {
		return settings, []string{err.Error()}
	}
	res, err := b.Resolve()
	if err != nil {
		return settings, []string{err.Error()}
	}

	for _, e := range res.Cfg.Settings {
		s := Setting{
			Name:         e.Name,
			Value:        e.Value,
			Description:  strings.TrimSpace(e.Description),
			Type:         e.SettingType.String(),
			State:        e.State.String(),
			Package:      pkgName(e.PackageDef),
			ValueRef:     e.ValueRefName,
			Choices:      e.ValidChoices,
			Restrictions: []Restriction{},
			History:      []SettingPoint{},
		}
		if s.Choices == nil {
			s.Choices = []string{}
		}
		for _, r := range e.Restrictions {
			switch r.Code {
			case syscfg.CFG_RESTRICTION_CODE_NOTNULL:
				s.Restrictions = append(s.Restrictions,
					Restriction{Type: "notnull"})
			case syscfg.CFG_RESTRICTION_CODE_EXPR:
				s.Restrictions = append(s.Restrictions,
					Restriction{Type: "expr", Expr: r.Expr})
			case syscfg.CFG_RESTRICTION_CODE_RANGE:
				s.Restrictions = append(s.Restrictions,
					Restriction{Type: "range", Expr: r.Expr})
			}
			// Choices are listed separately.
		}
		for _, p := range e.History {
			s.History = append(s.History,
				SettingPoint{Source: pkgName(p.Source), Value: p.Value})
		}
		if len(e.History) > 0 {
			s.Default = e.History[0].Value
		}
		settings = append(settings, s)
	}
	sort.Slice(settings, func(i, j int) bool {
		return settings[i].Name < settings[j].Name
	})

	errors := []string{}
	for _, line := range strings.Split(res.ErrorText(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			errors = append(errors, line)
		}
	}

	return settings, errors
}

func pkgName(lpkg *pkg.LocalPackage) string {
	if lpkg == nil {
		return "newt"
	}
	return lpkg.FullName()
}

// valueString formats a yml value, joining lists with spaces.
func valueString(v interface{}) string {
	if list, ok := v.([]interface{}); ok {
		strs := make([]string, len(list))
		for i, e := range list {
			strs[i] = cast.ToString(e)
		}
		return strings.Join(strs, " ")
	}
	return cast.ToString(v)
}

// repoUrl returns where the repo is downloaded from, without any credentials.
func repoUrl(r *repo.Repo) string {
	switch dl := r.Downloader().(type) {
	case *downloader.GithubDownloader:
		server := dl.Server
		if server == "" {
			server = "github.com"
		}
		return fmt.Sprintf("https://%s/%s/%s.git", server, dl.User, dl.Repo)
	case *downloader.GitDownloader:
		return dl.Url
	case *downloader.LocalDownloader:
		return dl.Path
	default:
		return ""
	}
}

func relPath(proj *project.Project, path string) string {
	rel, err := filepath.Rel(proj.Path(), path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// WritePorcelain writes the information as lines of the form
//
//	<kind> <name> <key> <value>
//
// where kind is "project", "repo", "target" or "package".  A target's
// variables have keys "var.<name>", the fields of its syscfg settings
// "syscfg.<setting>.<field>", and its syscfg errors "syscfg_error".  Keys of
// lists ("restriction", "history", "syscfg_error") are repeated, once per
// element, in order.  The first three fields never contain spaces; the value is the rest of the line, with
// backslashes and newlines escaped as "\\" and "\n".  Lines with empty values
// are left out.
func WritePorcelain(w io.Writer, info *Info) error {
	pw := &porcelainWriter{w: w}

	proj := info.Project.Name
	pw.line("project", proj, "format_version",
		fmt.Sprintf("%d", info.FormatVersion))
	pw.line("project", proj, "newt_version", info.NewtVersion)
	pw.line("project", proj, "path", info.Project.Path)

	for _, r := range info.Repos {
		pw.line("repo", r.Name, "path", r.Path)
		pw.line("repo", r.Name, "local", fmt.Sprintf("%t", r.Local))
		pw.line("repo", r.Name, "url", r.Url)
		pw.line("repo", r.Name, "requested", r.Requested)
		pw.line("repo", r.Name, "installed", r.Installed)
		pw.line("repo", r.Name, "commit", r.Commit)
		pw.line("repo", r.Name, "dirty", r.Dirty)
		pw.line("repo", r.Name, "needs_upgrade",
			fmt.Sprintf("%t", r.NeedsUpgrade))
		pw.line("repo", r.Name, "error", r.Error)
	}

	for _, t := range info.Targets {
		pw.line("target", t.Name, "repo", t.Repo)
		pw.line("target", t.Name, "path", t.Path)

		for _, k := range sortedKeys(t.Vars) {
			pw.line("target", t.Name, "var."+k, t.Vars[k])
		}
		for _, e := range t.SyscfgErrors {
			pw.line("target", t.Name, "syscfg_error", e)
		}
		for _, st := range t.Syscfg {
			key := "syscfg." + st.Name + "."
			pw.line("target", t.Name, key+"value", st.Value)
			pw.line("target", t.Name, key+"default", st.Default)
			pw.line("target", t.Name, key+"description", st.Description)
			pw.line("target", t.Name, key+"type", st.Type)
			pw.line("target", t.Name, key+"state", st.State)
			pw.line("target", t.Name, key+"package", st.Package)
			pw.line("target", t.Name, key+"value_ref", st.ValueRef)
			pw.line("target", t.Name, key+"choices",
				strings.Join(st.Choices, " "))
			for _, r := range st.Restrictions {
				pw.line("target", t.Name, key+"restriction",
					strings.TrimSpace(r.Type+" "+r.Expr))
			}
			for _, p := range st.History {
				pw.line("target", t.Name, key+"history",
					p.Source+" "+p.Value)
			}
		}
	}

	for _, p := range info.Packages {
		pw.line("package", p.Name, "type", p.Type)
		pw.line("package", p.Name, "repo", p.Repo)
		pw.line("package", p.Name, "path", p.Path)
		pw.line("package", p.Name, "description", p.Description)
	}

	return pw.err
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type porcelainWriter struct {
	w   io.Writer
	err error
}

var valueEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`)

func (pw *porcelainWriter) line(kind string, name string, key string,
	value string) {

	if pw.err != nil || value == "" {
		return
	}
	_, pw.err = fmt.Fprintf(pw.w, "%s %s %s %s\n", kind, name, key,
		valueEscaper.Replace(value))
}
