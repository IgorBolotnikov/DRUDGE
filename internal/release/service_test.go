package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

const (
	oldBinary = "old binary"
	newBinary = "new binary"
)

type fakeReleaseRepository struct {
	latestVersion string
	latestErr     error
	files         map[string][]byte
	downloaded    []string
}

func (f *fakeReleaseRepository) LatestVersion() (string, error) {
	return f.latestVersion, f.latestErr
}

func (f *fakeReleaseRepository) DownloadFile(version string, fileName string) ([]byte, error) {
	f.downloaded = append(f.downloaded, version+"/"+fileName)
	contents, ok := f.files[fileName]
	if !ok {
		return nil, fmt.Errorf("%s not found", fileName)
	}
	return contents, nil
}

func makeArchive(t *testing.T, entryName string, contents string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, body := range map[string]string{"README.md": "readme", entryName: contents} {
		header := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func checksumLine(archive []byte, name string) string {
	sum := sha256.Sum256(archive)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func currentArchiveName() string {
	return fmt.Sprintf(archiveNamePattern, runtime.GOOS, runtime.GOARCH)
}

// releaseFiles returns the files of a good release that ships newBinary.
func releaseFiles(t *testing.T) map[string][]byte {
	t.Helper()
	archive := makeArchive(t, binaryName, newBinary)
	checksums := checksumLine([]byte("other"), "drg_plan9_mips.tar.gz") + checksumLine(archive, currentArchiveName())
	return map[string][]byte{currentArchiveName(): archive, checksumsFileName: []byte(checksums)}
}

func TestUpdate(t *testing.T) {
	cases := []struct {
		name           string
		currentVersion string
		latestVersion  string
		latestErr      error
		files          func(t *testing.T) map[string][]byte
		wantResult     UpdateResult
		wantErr        string
		wantBinary     string
	}{
		{
			name:           "replaces an older binary",
			currentVersion: "v0.1.0",
			latestVersion:  "v0.1.1",
			files:          releaseFiles,
			wantResult:     UpdateResult{PreviousVersion: "v0.1.0", Version: "v0.1.1"},
			wantBinary:     newBinary,
		},
		{
			name:           "compares versions as numbers",
			currentVersion: "v0.9.0",
			latestVersion:  "v0.10.0",
			files:          releaseFiles,
			wantResult:     UpdateResult{PreviousVersion: "v0.9.0", Version: "v0.10.0"},
			wantBinary:     newBinary,
		},
		{
			name:           "leaves the latest release alone",
			currentVersion: "v0.1.1",
			latestVersion:  "v0.1.1",
			wantResult:     UpdateResult{PreviousVersion: "v0.1.1", Version: "v0.1.1", IsUpToDate: true},
			wantBinary:     oldBinary,
		},
		{
			name:           "leaves a newer binary alone",
			currentVersion: "v0.2.0",
			latestVersion:  "v0.1.1",
			wantResult:     UpdateResult{PreviousVersion: "v0.2.0", Version: "v0.2.0", IsUpToDate: true},
			wantBinary:     oldBinary,
		},
		{
			name:           "refuses a dev build",
			currentVersion: "dev",
			latestVersion:  "v0.1.1",
			wantErr:        "drg dev was not installed from a release",
			wantBinary:     oldBinary,
		},
		{
			name:           "refuses a dirty local build",
			currentVersion: "v0.1.0+dirty",
			latestVersion:  "v0.1.1",
			wantErr:        "drg v0.1.0+dirty was not installed from a release",
			wantBinary:     oldBinary,
		},
		{
			name:           "refuses a pseudo-version",
			currentVersion: "v0.1.1-0.20260925120000-917bc7a0e1f2",
			latestVersion:  "v0.1.1",
			wantErr:        "was not installed from a release",
			wantBinary:     oldBinary,
		},
		{
			name:           "reports a failed latest release lookup",
			currentVersion: "v0.1.0",
			latestErr:      errors.New("connection refused"),
			wantErr:        "could not look up the latest release: connection refused",
			wantBinary:     oldBinary,
		},
		{
			name:           "refuses an unexpected latest tag",
			currentVersion: "v0.1.0",
			latestVersion:  "nightly",
			wantErr:        `the latest release has an unexpected tag "nightly"`,
			wantBinary:     oldBinary,
		},
		{
			name:           "reports a missing archive",
			currentVersion: "v0.1.0",
			latestVersion:  "v0.1.1",
			files: func(t *testing.T) map[string][]byte {
				files := releaseFiles(t)
				delete(files, currentArchiveName())
				return files
			},
			wantErr:    "could not download " + currentArchiveName() + " of v0.1.1",
			wantBinary: oldBinary,
		},
		{
			name:           "refuses a checksum mismatch",
			currentVersion: "v0.1.0",
			latestVersion:  "v0.1.1",
			files: func(t *testing.T) map[string][]byte {
				files := releaseFiles(t)
				files[currentArchiveName()] = makeArchive(t, binaryName, "tampered binary")
				return files
			},
			wantErr:    "checksum of " + currentArchiveName() + " is",
			wantBinary: oldBinary,
		},
		{
			name:           "refuses an archive missing from the checksums",
			currentVersion: "v0.1.0",
			latestVersion:  "v0.1.1",
			files: func(t *testing.T) map[string][]byte {
				files := releaseFiles(t)
				files[checksumsFileName] = []byte(checksumLine([]byte("other"), "drg_plan9_mips.tar.gz"))
				return files
			},
			wantErr:    "checksums.txt has no entry for " + currentArchiveName(),
			wantBinary: oldBinary,
		},
		{
			name:           "refuses an archive without the binary",
			currentVersion: "v0.1.0",
			latestVersion:  "v0.1.1",
			files: func(t *testing.T) map[string][]byte {
				archive := makeArchive(t, "LICENSE", "license")
				return map[string][]byte{
					currentArchiveName(): archive,
					checksumsFileName:    []byte(checksumLine(archive, currentArchiveName())),
				}
			},
			wantErr:    "the archive has no drg binary",
			wantBinary: oldBinary,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			binaryPath := filepath.Join(t.TempDir(), binaryName)
			if err := os.WriteFile(binaryPath, []byte(oldBinary), 0o755); err != nil {
				t.Fatal(err)
			}
			repo := &fakeReleaseRepository{latestVersion: testCase.latestVersion, latestErr: testCase.latestErr}
			if testCase.files != nil {
				repo.files = testCase.files(t)
			}
			service := NewReleaseService(repo, common.NewLogger(""))

			result, err := service.Update(testCase.currentVersion, binaryPath)

			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				wantResult := testCase.wantResult
				wantResult.BinaryPath = binaryPath
				if *result != wantResult {
					t.Errorf("result = %+v, want %+v", *result, wantResult)
				}
			}

			gotBinary, err := os.ReadFile(binaryPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotBinary) != testCase.wantBinary {
				t.Errorf("binary = %q, want %q", gotBinary, testCase.wantBinary)
			}
			entries, err := os.ReadDir(filepath.Dir(binaryPath))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("expected only the binary to be left in its directory, got %d entries", len(entries))
			}
		})
	}
}

func TestUpdate_MakesTheBinaryExecutable(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(binaryPath, []byte(oldBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := &fakeReleaseRepository{latestVersion: "v0.1.1", files: releaseFiles(t)}
	service := NewReleaseService(repo, common.NewLogger(""))

	if _, err := service.Update("v0.1.0", binaryPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != executableFilePerm {
		t.Errorf("binary mode = %v, want %v", info.Mode().Perm(), os.FileMode(executableFilePerm))
	}
	wantDownloads := []string{"v0.1.1/" + currentArchiveName(), "v0.1.1/" + checksumsFileName}
	if strings.Join(repo.downloaded, ",") != strings.Join(wantDownloads, ",") {
		t.Errorf("downloaded %v, want %v", repo.downloaded, wantDownloads)
	}
}

func TestUpdate_RefusesAnUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to any directory")
	}
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte(oldBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	repo := &fakeReleaseRepository{latestVersion: "v0.1.1", files: releaseFiles(t)}
	service := NewReleaseService(repo, common.NewLogger(""))

	_, err := service.Update("v0.1.0", binaryPath)

	if err == nil || !strings.Contains(err.Error(), "could not write to "+dir) {
		t.Errorf("expected a write error for %s, got %v", dir, err)
	}
}
