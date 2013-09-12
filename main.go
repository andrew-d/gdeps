package main

import (
    "bufio"
    "bytes"
    "flag"
    "fmt"
    "go/build"
    "os"
    "os/exec"
    "path"
    "strings"
)

type VcsInfo struct {
    Name string
    Tool string
    Dir  string

    CheckoutCommand string
}

var vcsGit = &VcsInfo{
    Name: "Git",
    Tool: "git",
    Dir:  ".git",

    CheckoutCommand: "checkout %s",
}

var vcsHg = &VcsInfo{
    Name: "Mercurial",
    Tool: "hg",
    Dir:  ".hg",

    CheckoutCommand: "update -C -r %s",
}

var vcsSvn = &VcsInfo{
    Name: "Subversion",
    Tool: "svn",
    Dir:  ".svn",

    CheckoutCommand: "update -r %s",
}

var vcsBzr = &VcsInfo{
    Name: "Bazaar",
    Tool: "bzr",
    Dir:  ".bzr",

    CheckoutCommand: "pull --overwrite -r %s",
}

var vcsList = []*VcsInfo{
    vcsGit,
    vcsHg,
    vcsSvn,
    vcsBzr,
}

var (
    flagGodepsFile string
    workingDir     string
)

func Fatalf(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, format, args...)
    os.Exit(1)
}

func Warnf(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, format, args...)
}

func main() {
    var err error

    // Verify requirements
    workingDir, err = os.Getwd()
    if err != nil {
        Fatalf(">> could not get current dir\n")
    }
    if _, err = exec.LookPath("go"); err != nil {
        Fatalf(">> go tool not found in PATH\n")
    }
    if pth := os.Getenv("GOPATH"); pth == "" {
        Fatalf(">> GOPATH not set")
    }

    // Parse command line
    flag.StringVar(&flagGodepsFile,
        "f",
        path.Join(workingDir, "Godeps"),
        "Godeps file path (default: current dir)")
    flag.Parse()

    // Do the real work
    file, err := os.Open(flagGodepsFile)
    if err != nil {
        Fatalf(">> could not open Godeps file: %s\n", flagGodepsFile)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    i := 0
    for scanner.Scan() {
        i += 1
        processLine(scanner.Text(), i)
    }
}

func processLine(line string, linenum int) {
    // Trim comments and whitespace.
    spl := strings.Split(line, "#")
    line = strings.TrimSpace(spl[0])
    if len(line) == 0 {
        return
    }

    args := strings.Fields(line)
    if len(args) < 2 {
        Warnf(">> bad line %d, skipping...\n", linenum)
        return
    }

    // Get the package.
    fmt.Printf(">> getting package %s\n", args[0])
    cmd := exec.Command("go", "get", "-u", "-d", args[0])
    if err := cmd.Run(); err != nil {
        Warnf(">> error getting: %s\n", args[0])
        return
    }

    // Get the path that this will be saved to.
    pkg, err := build.Import(args[0], workingDir, build.FindOnly)
    if err != nil {
        Warnf(">> could not get information about %s - version not set\n", args[0])
        return
    }

    err = os.Chdir(pkg.Dir)
    if err != nil {
        Warnf(">> could not change to package dir: %s\n", pkg.Dir)
        return
    }

    // Determine the type of VCS.
    for _, x := range vcsList {
        exists, err := dirExists(x.Dir)
        if err != nil {
            Warnf(">> could not check for dir '%s' in %s\n", x.Dir, pkg.Dir)
            continue
        }

        if exists {
            fmt.Printf(">> setting package %s (%s) to version: %s\n", args[0], x.Name, args[1])

            if _, err = exec.LookPath(x.Tool); err != nil {
                Warnf(">> tool '%s' was not found in PATH\n", x.Tool)
                return
            }

            args := strings.Fields(fmt.Sprintf(x.CheckoutCommand, args[1]))
            cmd = exec.Command(x.Tool, args...)
            if out, err := cmd.CombinedOutput(); err != nil {
                Warnf(">> error setting version: %s\n", err)
                Warnf(">> output: %s\n", formatOutput(out))
                return
            }
            return
        }
    }

    Warnf(">> unknown VCS type for package %s\n", args[0])
}

func dirExists(dirname string) (bool, error) {
    fi, err := os.Stat(dirname)
    if err != nil {
        // Only actually return an error if this is NOT a "does not exist"
        // error.
        if os.IsNotExist(err) {
            return false, nil
        }
        return false, err
    }

    return fi.IsDir(), nil
}

func formatOutput(in []byte) []byte {
    return bytes.Join(bytes.Split(in, []byte{'\n'}), []byte("\n     "))
}
