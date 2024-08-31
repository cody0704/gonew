// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gonew starts a new Go module by cloning a template repository.
//
// Usage:
//
//	gonew src repo[@version] [dstmod [dir]]
//
// Gonew clones the src repo, changing its module path to dstmod.
// It writes that new module to a new directory named by dir.
// If dir already exists, it must be an empty directory.
// If dir is omitted, gonew uses ./elem where elem is the final path element of dstmod.
//
// This command is highly experimental and subject to change.
//
// # Example
//
// To install gonew:
//
//	go install github.com/your-username/gonew@latest
//
// To clone the basic command-line program template github.com/example/hello
// as your.domain/myprog, in the directory ./myprog:
//
//	gonew github.com/example/hello your.domain/myprog
//
// To clone the latest copy of the github.com/rsc/quote repository, keeping that module path,
// into ./quote:
//
//	gonew github.com/rsc/quote
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cody0704/gonew/internal/edit"
	"golang.org/x/mod/modfile"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gonew src repo[@version] [dstmod [dir]]\n")
	fmt.Fprintf(os.Stderr, "See https://pkg.go.dev/golang.org/x/tools/cmd/gonew.\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("gonew: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 || len(args) > 3 {
		usage()
	}

	srcRepo := args[0]
	srcRepoVers := ""
	if strings.Contains(srcRepo, "@") {
		srcRepoVers = strings.Split(srcRepo, "@")[1]
	}
	_ = srcRepoVers

	dstRepo := srcRepo
	if len(args) >= 2 {
		dstRepo = args[1]
	}
	dstRepoNameSlice := strings.Split(dstRepo, "/")
	dstRepoName := dstRepoNameSlice[len(dstRepoNameSlice)-1]
	_ = dstRepoName
	// github.com/<org>/<project> -> github.com:<org>/<project>
	githubURL := strings.Replace(srcRepo, "/", ":", 1)
	_ = githubURL

	// Clone the source repo
	giturl := fmt.Sprintf("%s@%s.git", "git", githubURL)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "clone", giturl, dstRepoName)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("git clone %s: %v\n%s%s", srcRepo, err, stderr.Bytes(), stdout.Bytes())
	}

	// get now working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}

	dst := path.Join(wd, dstRepoName)

	var gitdir string = ""
	// Change project go module name to dstRepo
	filepath.WalkDir(dst, func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}

		if d.IsDir() && d.Name() == ".git" {
			gitdir = src

			return nil
		}

		// check *.go files
		// fix go file
		isRoot := !strings.Contains(src, string(filepath.Separator))
		if strings.HasSuffix(src, ".go") {
			data, err := os.ReadFile(src)
			if err != nil {
				log.Fatal(".go err:", err)
			}

			data = fixGo(data, src, srcRepo, dstRepo, isRoot)

			if err := os.WriteFile(src, data, 0666); err != nil {
				log.Fatal("write:", err)
			}
		}

		if strings.HasSuffix(src, "go.mod") {
			data, err := os.ReadFile(src)
			if err != nil {
				log.Fatal(".go err:", err)
			}
			data = fixGoMod(data, dstRepo)

			if err := os.WriteFile(src, data, 0666); err != nil {
				log.Fatal("write2:", err)
			}
		}

		return nil
	})

	// Remove .git directory
	if gitdir != "" {
		if err := os.RemoveAll(gitdir); err != nil {
			log.Fatal("remove .git:", err)
		}
	}
}

// fixGo rewrites the Go source in data to replace srcMod with dstMod.
// isRoot indicates whether the file is in the root directory of the module,
// in which case we also update the package name.
func fixGo(data []byte, file string, srcMod, dstMod string, isRoot bool) []byte {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, data, parser.ImportsOnly)
	if err != nil {
		log.Fatalf("parsing source module:\n%s", err)
	}

	buf := edit.NewBuffer(data)
	at := func(p token.Pos) int {
		return fset.File(p).Offset(p)
	}

	srcName := path.Base(srcMod)
	dstName := path.Base(dstMod)
	if isRoot {
		if name := f.Name.Name; name == srcName || name == srcName+"_test" {
			dname := dstName + strings.TrimPrefix(name, srcName)
			if !token.IsIdentifier(dname) {
				log.Fatalf("%s: cannot rename package %s to package %s: invalid package name", file, name, dname)
			}
			buf.Replace(at(f.Name.Pos()), at(f.Name.End()), dname)
		}
	}

	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		if path == srcMod {
			if srcName != dstName && spec.Name == nil {
				// Add package rename because source code uses original name.
				// The renaming looks strange, but template authors are unlikely to
				// create a template where the root package is imported by packages
				// in subdirectories, and the renaming at least keeps the code working.
				// A more sophisticated approach would be to rename the uses of
				// the package identifier in the file too, but then you have to worry about
				// name collisions, and given how unlikely this is, it doesn't seem worth
				// trying to clean up the file that way.
				buf.Insert(at(spec.Path.Pos()), srcName+" ")
			}
			// Change import path to dstMod
			buf.Replace(at(spec.Path.Pos()), at(spec.Path.End()), strconv.Quote(dstMod))
		}
		if strings.HasPrefix(path, srcMod+"/") {
			// Change import path to begin with dstMod
			buf.Replace(at(spec.Path.Pos()), at(spec.Path.End()), strconv.Quote(strings.Replace(path, srcMod, dstMod, 1)))
		}
	}
	return buf.Bytes()
}

// fixGoMod rewrites the go.mod content in data to replace srcMod with dstMod
// in the module path.
func fixGoMod(data []byte, dstMod string) []byte {
	f, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		log.Fatalf("parsing source module:\n%s", err)
	}
	f.AddModuleStmt(dstMod)
	new, err := f.Format()
	if err != nil {
		return data
	}
	return new
}
