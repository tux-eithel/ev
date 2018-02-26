package ev

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	startLog = "HEADER:"

	commitHash  = "%H"
	authorName  = "%an"
	authorEmail = "%ae"
	authorDate  = "%ad"

	committerName  = "%cn"
	committerEmail = "%ce"
	committerDate  = "%cd"

	endLog = "EV_BODY_END"

	separator = "|"

	localRFC1123Z = "Mon, _2 Jan 2006 15:04:05 -0700"
)

// Commit holds an entry inside the git log output.
type Commit struct {
	SHA            string
	AuthorName     string
	AuthorEmail    string
	AuthorDate     time.Time
	CommitterName  string
	CommitterEmail string
	CommitterDate  time.Time
	Msg            string
	Diff           string
	Changes        int
}

// logReader executes the `git log -L:<re>:<fn>` command with a custom format
// and returns an io.Reader which can read from the output.
func logReader(re, file string) (io.Reader, error) {

	logCommand := []string{
		"log",
		"--date=rfc",
		"--pretty=format:" + startLog + ":" + strings.Join([]string{commitHash, authorName, authorEmail, authorDate, committerName, committerEmail, committerDate}, separator) + "%n%s%n" + endLog,
	}
	logCommand = append(logCommand, fmt.Sprintf("-L %s:%s", re, file))

	cmd := exec.Command("git", logCommand...)
	cmd.Dir = filepath.Dir(file)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		gitError := stderr.String()

		if strings.Contains(re, ",") {

			rows := recoverError(gitError)
			if rows != "" {
				split := strings.Split(re, ",")
				return logReader(fmt.Sprintf("%s,%s", split[0], rows), file)
			}
		}

		baseErrorStrs := `error running command: %s`
		baseErrorVars := []interface{}{err}
		if gitError != "" {
			baseErrorStrs += `; output command: %s`
			baseErrorVars = append(baseErrorVars, gitError)
		}

		return nil, fmt.Errorf(baseErrorStrs, baseErrorVars...)
	}
	return &stdout, nil
}

// Log parses the result of the `git log -L:<re>:<fn>` command and returns
// a slice of commits. They are ordered in descending chronological order
// and show the history of the function (or regexp) `re` inside the file `fn`.
func Log(re, fn string) ([]*Commit, error) {
	r, err := logReader(re, fn)
	if err != nil {
		return nil, err
	}
	scn := bufio.NewScanner(r)
	list := make([]*Commit, 0)
	var (
		c    *Commit
		diff bytes.Buffer
		msg  bytes.Buffer
	)
	readingDiff := false
	for scn.Scan() {
		line := scn.Text()
		if strings.HasPrefix(line, startLog) {
			readingDiff = false
			if c != nil {
				c.Diff = diff.String()
				c.Msg = msg.String()
				list = append(list, c)
			}
			c = new(Commit)
			err := readHeader(line[len(startLog):], c)
			if err != nil {
				return nil, err
			}
			diff.Truncate(0)
			msg.Truncate(0)
			continue
		}
		if line == endLog {
			readingDiff = true
			continue
		}
		if readingDiff {
			if len(line) >= 1 && (line[0] == '-' || line[0] == '+') {
				c.Changes++
			}
			diff.WriteString(line)
			diff.WriteString("\r\n")
		} else {
			msg.WriteString(line)
			msg.WriteString("\r\n")
		}
	}
	if err := scn.Err(); err != nil {
		return nil, fmt.Errorf("parse: %s", err)
	}
	// append the last item!
	if c != nil {
		c.Diff = diff.String()
		c.Msg = msg.String()
		list = append(list, c)
	}
	return list, nil
}

// readHeader reads a git log header into c.
func readHeader(line string, c *Commit) error {
	p := strings.Split(line, separator)
	if len(p) != 7 {
		return fmt.Errorf("bad header: %s", line)
	}

	aDate, err := time.Parse(localRFC1123Z, p[3])
	if err != nil {
		return fmt.Errorf("unable to parse Author date: %s", err)
	}
	cDate, err := time.Parse(localRFC1123Z, p[6])
	if err != nil {
		return fmt.Errorf("unable to parse Committer date: %s", err)
	}

	c.SHA, c.AuthorName, c.AuthorEmail, c.AuthorDate,
		c.CommitterName, c.CommitterEmail, c.CommitterDate =
		p[0], p[1], p[2], aDate, p[4], p[5], cDate
	return nil
}

// recoverError tries to match "fatal: file <filename> has only 395 lines" git error string
func recoverError(e string) string {
	r := regexp.MustCompile(`has only (\d+) lines`)
	a := r.FindStringSubmatch(e)
	if len(a) == 2 {
		return a[1]
	}
	return ""
}
