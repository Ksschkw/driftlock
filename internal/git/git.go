package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetStagedDiff returns the output of 'git diff --cached' (unified diff).
func GetStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "-U5")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff failed: %v\noutput: %s", err, out.String())
	}
	return out.String(), nil
}

// GetFileContentAtHEAD returns the content of a file at HEAD.
func GetFileContentAtHEAD(filePath string) (string, error) {
	cmd := exec.Command("git", "show", "HEAD:"+filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git show failed for %s: %v\noutput: %s", filePath, err, out.String())
	}
	return out.String(), nil
}

// GetStagedFileContent returns the content of a staged file.
func GetStagedFileContent(filePath string) (string, error) {
	cmd := exec.Command("git", "show", ":"+filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git show (staged) failed for %s: %v\noutput: %s", filePath, err, out.String())
	}
	return out.String(), nil
}

// ListStagedFiles returns a list of files that are staged for commit.
func ListStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %v\noutput: %s", err, out.String())
	}
	files := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}