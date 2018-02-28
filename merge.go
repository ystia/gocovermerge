package main // import "github.com/ystia/gocovermerge"

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"golang.org/x/tools/cover"

	"github.com/pkg/errors"
)

var output = flag.String("output", "coverage.txt", "path to the merged output profile")
var keepGenerated = flag.Bool("keep", false, "keep intermediary generated files")

// concatProfiles merge all the profiles files into a single temporary one
//
// returned file is already closed.
func concatProfiles(profilesPath []string) (*os.File, error) {
	finalMode := ""
	f, err := ioutil.TempFile("", "mergeprofiles")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary file for merging profiles")
	}
	defer f.Close()

	for _, fileName := range profilesPath {
		pf, err := os.Open(fileName)
		if err != nil {
			return f, errors.Wrapf(err, "failed to open profile file %q for reading", fileName)
		}
		// Use inline func for defering close of pf
		err = func() error {
			defer pf.Close()
			buf := bufio.NewReader(pf)
			s := bufio.NewScanner(buf)
			mode := ""
			for s.Scan() {
				line := strings.TrimSpace(s.Text())
				if line == "" {
					continue
				}
				if mode == "" {
					const p = "mode: "
					if !strings.HasPrefix(line, p) || line == p {
						return errors.Errorf("bad mode line: %v", line)
					}
					mode = line[len(p):]
					if finalMode == "" {
						finalMode = mode
						f.WriteString(line)
					} else if finalMode != mode {
						return errors.Errorf("Can't merge profiles of different modes %q and %q (comming from %q)", finalMode, mode, fileName)
					}
					continue
				}
				f.WriteString("\n" + line)
			}
			return nil
		}()
		if err != nil {
			return f, err
		}
	}
	return f, nil
}

// mergeProfiles use cover package to parse and merge profiles. Then it write the result into the
func mergeProfiles(f *os.File) error {
	// parse profiles does the job of merging paths
	profiles, err := cover.ParseProfiles(f.Name())
	if err != nil {
		return errors.Wrapf(err, "failed to parse temporary profile %q", f.Name())
	}
	out, err := os.Create(*output)
	if err != nil {
		return errors.Wrapf(err, "failed to create result file %q", *output)
	}
	defer out.Close()
	for i, profile := range profiles {
		if i == 0 {
			_, err = out.WriteString("mode: " + profile.Mode + "\n")
			if err != nil {
				return errors.Wrapf(err, "failed to write to file %q", out.Name())
			}
		}
		for _, block := range profile.Blocks {
			_, err := out.WriteString(fmt.Sprintf("%s:%d.%d,%d.%d %d %d\n", profile.FileName, block.StartLine, block.StartCol, block.EndLine, block.EndCol, block.NumStmt, block.Count))
			if err != nil {
				return errors.Wrapf(err, "failed to write to file %q", out.Name())
			}
		}
	}
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	fmt.Printf("Merging profiles: %s\n", strings.Join(args, " "))
	f, err := concatProfiles(args)
	if f != nil && !*keepGenerated {
		defer os.Remove(f.Name())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	err = mergeProfiles(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	fmt.Printf("cover profiles merged into: %q\n", *output)
}
