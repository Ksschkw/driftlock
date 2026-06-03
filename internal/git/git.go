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
// If the file does not exist in HEAD (new file), returns empty string with no error.
func GetFileContentAtHEAD(filePath string) (string, error) {
	cmd := exec.Command("git", "show", "HEAD:"+filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		errStr := out.String()
		if strings.Contains(errStr, "exists on disk, but not in 'HEAD'") ||
			strings.Contains(errStr, "bad revision") {
			return "", nil
		}
		return "", fmt.Errorf("git show failed for %s: %v\noutput: %s", filePath, err, errStr)
	}
	return out.String(), nil
}

// GetStagedFileContent returns the content of a staged file.
// If the file has been deleted, returns empty string with no error.
func GetStagedFileContent(filePath string) (string, error) {
	// Check if the file is deleted in the staging area
	cmdStatus := exec.Command("git", "diff", "--cached", "--name-status", "--", filePath)
	outStatus, err := cmdStatus.Output()
	if err == nil && strings.HasPrefix(string(outStatus), "D") {
		return "", nil // deleted file, no content
	}

	cmd := exec.Command("git", "show", ":"+filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		errStr := out.String()
		if strings.Contains(errStr, "does not exist") {
			return "", nil
		}
		return "", fmt.Errorf("git show (staged) failed for %s: %v\noutput: %s", filePath, err, errStr)
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

// ListTrackedFiles returns all files tracked by Git in the current working directory.
func ListTrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files failed: %v\noutput: %s", err, out.String())
	}
	files := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}
