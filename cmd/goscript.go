// Copyright 2010  The "goscript" Authors
//
// Use of this source code is governed by the Simplified BSD License
// that can be found in the LICENSE file.
//
// This software is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES
// OR CONDITIONS OF ANY KIND, either express or implied. See the License
// for more details.

package main

import (
	"exec"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"io"
	"bufio"
	"regexp"
)

// Error exit status
const ERROR = 1

// Environment variables passed at running a command
var ENVIRON []string


// === Flags
// ===

var fShared = flag.Bool("shared", false,
	"whether the script is used on a mixed network of machines or   "+
	"systems from a shared filesystem")


func usage() {
	flag.PrintDefaults()
	os.Exit(ERROR)
}
// ===

func error(s string) {
	fmt.Printf("Error: %s\n", s)
	os.Exit(1)
}

func warn(s string) {
	fmt.Printf("Warnning: %s\n", s)
}

func isource(dst , src string) (refFiles []string) {
	refFiles = make([]string, 0) 
	file1, err := os.Open(src, os.O_RDONLY, 0)
	if err != nil {
		error(fmt.Sprintf("Can't open %s", src))
	}
	defer file1.Close()

	os.Remove(dst)
	file2, err := os.Open(dst, os.O_WRONLY | os.O_CREAT, 0644)
	if err != nil {
		error(fmt.Sprintf("Can't open %s", flag.Args()[1]))
	}
	defer file2.Close()

	bufFile1 := bufio.NewReader(file1)
	bufFile2 := bufio.NewWriter(file2)
	defer bufFile2.Flush()
	head, _ := bufFile1.ReadString('\n')
	if len(head) >= 2 && head[0:2] != "#!" {
		//error("First Line: " + head)
		bufFile2.WriteString(head + "\n")
	} else {
		bufFile2.WriteString("\n")
	}
    refline, _ := bufFile1.ReadString('\n')
    if len(refline) >= 5 && refline[0:5] != "///<>" {
        bufFile2.WriteString(refline + "\n")
    } else {
        refFiles = regexp.MustCompile("[_a-zA-Z]+\\.go").FindAllString(refline, 1000)
        bufFile2.WriteString(refline + "\n")
    }
	io.Copy(bufFile2, bufFile1)
	return
}


func main() {
	var binaryDir, binaryPath string

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, `Tool to run Go scripts

== Usage
Insert "#!/usr/bin/goscript" in the head of the Go script

=== In shared filesystem
  $ /usr/bin/goscript -shared /path/to/shared-fs/file.go

Flags:
`)
		usage()
	}

	scriptPath := flag.Args()[0] // Relative path
	scriptDir, scriptName := path.Split(scriptPath)

	if !*fShared {
		binaryDir = path.Join(scriptDir, ".go")
	} else {
		binaryDir = path.Join(scriptDir, ".go", runtime.GOOS+"_"+runtime.GOARCH)
	}
	ext := path.Ext(scriptName)
	binaryPath = path.Join(binaryDir, scriptName[0:len(scriptName) - len(ext)])
	// warn(ext)
	// warn(binaryDir)
	// warn(binaryPath)

	// Check directory
	if ok := Exist(binaryDir); !ok {
		if err := os.MkdirAll(binaryDir, 0750); err != nil {
			fmt.Fprintf(os.Stderr, "Could not make directory: %s\n", err)
			os.Exit(ERROR)
		}
	}

	// Run the executable, if exist and it has not been modified
	if ok := Exist(binaryPath); ok {
		scriptMtime := getTime(scriptPath)
		binaryMtime := getTime(binaryPath)

		if scriptMtime <= binaryMtime {
			goto _run
		}
	}

	// Check script extension
	/*if ext != ".go" && ext != ".gos" && ext != "" {
		fmt.Fprintf(os.Stderr, "Wrong extension! It must be \".go\" or \".gos\"\n")
		os.Exit(ERROR)
	}*/

	// === Compile and link
	// scriptMtime := getTime(scriptPath)
	
	//comment(scriptPath)
	iscriptPath := scriptPath + ".i"
	refFiles := isource(iscriptPath, scriptPath)
	compiler, linker, archExt := toolchain()

	ENVIRON = os.Environ()
	objectPath := path.Join(binaryDir, "_go_."+archExt)

	cmdArgs := []string{path.Base(compiler), "-o", objectPath, iscriptPath}
	cmdArgs = append(cmdArgs, refFiles...)
	exitCode := run(compiler, cmdArgs, "")
	//comment(scriptPath, false)
	os.Remove(iscriptPath)
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	cmdArgs = []string{path.Base(linker), "-o", binaryPath, objectPath}
	if exitCode = run(linker, cmdArgs, ""); exitCode != 0 {
		os.Exit(exitCode)
	}

	// Set mtime of executable just like the source file
	// setTime(scriptPath, scriptMtime)
	// setTime(binaryPath, scriptMtime)

	// Cleaning
	/*if err := os.Remove(objectPath); err != nil {
		fmt.Fprintf(os.Stderr, "Could not remove: %s\n", err)
		os.Exit(ERROR)
	}*/

_run:
	// for a,v := range flag.Args() {
	// 	fmt.Print(a)
	// 	fmt.Println(v)
	// }
	// normArgs := make([]string, len(flag.Args()) + 1)
	// // normArgs = append(normArgs, binaryPath)
	// normArgs = append(normArgs, flag.Args()...)
	exitCode = run(binaryPath, /*normArgs*/ flag.Args()[:], scriptDir)
	os.Exit(exitCode)
}


// === Utility
// ===

// Base to access to "mtime" of given file.
func _time(filename string, mtime int64) int64 {
	info, err := os.Stat(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not access: %s\n", err)
		os.Exit(ERROR)
	}

	if mtime != 0 {
		info.Mtime_ns = mtime
		return 0
	}
	return info.Mtime_ns
}

func getTime(filename string) int64 {
	return _time(filename, 0)
}

func setTime(filename string, mtime int64) {
	_time(filename, mtime)
}


// Checks if exist a file.
func Exist(name string) bool {
	if _, err := os.Stat(name); err == nil {
		return true
	}
	return false
}

// Executes a command and returns its exit code.
func run(cmd string, args []string, dir string) int {
	// Execute the command
	process, err := exec.Run(cmd, args, ENVIRON, dir,
		exec.PassThrough, exec.PassThrough, exec.PassThrough)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not execute: \"%s\"\n",
			strings.Join(args, " "))
		os.Exit(ERROR)
	}

	// Wait for command completion
	message, err := process.Wait(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not wait for: \"%s\"\n",
			strings.Join(args, " "))
		os.Exit(ERROR)
	}

	return message.ExitStatus()
}

// Gets the toolchain.
func toolchain() (compiler, linker, archExt string) {
	arch_ext := map[string]string{
		"amd64": "6",
		"386":   "8",
		"arm":   "5",
	}

	// === Environment variables
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		goroot = os.Getenv("GOROOT_FINAL")
		if goroot == "" {
			fmt.Fprintf(os.Stderr, "Environment variable GOROOT neither"+
				" GOROOT_FINAL has been set\n")
			os.Exit(ERROR)
		}
	}

	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gobin = goroot + "/bin"
	}

	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	// === Set toolchain
	archExt, ok := arch_ext[goarch]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown GOARCH: %s\n", goarch)
		os.Exit(ERROR)
	}

	compiler = path.Join(gobin, archExt+"g")
	linker = path.Join(gobin, archExt+"l")
	return
}

