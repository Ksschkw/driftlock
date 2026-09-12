package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The installers download a fixed asset name per platform. If the release
// workflow stops producing one of them, installation breaks for that platform
// with a 404 that is hard to attribute.
func TestReleaseBuildsEveryInstallerAsset(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "release.yml")
	// Asset names are assembled from the os/arch variables, so assert the inputs
	// the installer's naming scheme depends on.
	for _, target := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"} {
		if !strings.Contains(workflow, target) {
			t.Errorf("release workflow does not build %s", target)
		}
	}
	if !strings.Contains(workflow, `sha256sum "${name}" > "${name}.sha256"`) {
		t.Error("release workflow does not publish a per-asset .sha256")
	}
	if !strings.Contains(workflow, "checksums.txt") {
		t.Error("release workflow does not publish checksums.txt")
	}
	if !strings.Contains(workflow, "sha256sum -c checksums.txt") {
		t.Error("release workflow does not verify its own checksums before publishing")
	}
}

// The installers ask for the exact asset names the release produces.
func TestInstallerAssetNamesMatchRelease(t *testing.T) {
	sh := readRepoFile(t, "install.sh")
	if !strings.Contains(sh, `TARGET="${BIN_NAME}-${os}-${arch}"`) {
		t.Error("install.sh no longer derives driftlock-<os>-<arch>")
	}
	ps := readRepoFile(t, "install.ps1")
	if !strings.Contains(ps, `"${BinName}-windows-${Arch}.exe"`) {
		t.Error("install.ps1 no longer derives driftlock-windows-<arch>.exe")
	}
}

// A checksum that is present but wrong must abort the install. Installing the
// binary anyway defeats the purpose of publishing checksums.
func TestInstallersRefuseMismatchedChecksums(t *testing.T) {
	sh := readRepoFile(t, "install.sh")
	if n := strings.Count(sh, "Refusing to install"); n < 2 {
		t.Errorf("install.sh has %d refusal(s), want one per checksum source (per-asset and checksums.txt)", n)
	}
	ps := readRepoFile(t, "install.ps1")
	if n := strings.Count(ps, "Refusing to install"); n < 2 {
		t.Errorf("install.ps1 has %d refusal(s), want one per checksum source", n)
	}
}
