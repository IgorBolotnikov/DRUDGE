package release

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

const (
	binaryName          = "drg"
	archiveNamePattern  = "drg_%s_%s.tar.gz"
	checksumsFileName   = "checksums.txt"
	tempBinaryPattern   = ".drg-update-*"
	executableFilePerm  = 0o755
	versionNumberFields = 3
)

// releaseVersionPattern matches the tags the release build stamps into drg.
// A local build carries a suffix like +dirty and a go install from a commit
// carries a pseudo-version, and neither matches.
var releaseVersionPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

type ReleaseService struct {
	repo ReleaseRepository
	log  *common.Logger
}

func NewReleaseService(repo ReleaseRepository, log *common.Logger) *ReleaseService {
	return &ReleaseService{repo: repo, log: log}
}

// Update replaces the binary at binaryPath with the newest release when that
// release is newer than currentVersion. It refuses a currentVersion that is
// not a release tag, and it refuses an archive whose checksum does not match
// the checksums file of the release. The binary is swapped with a rename, so
// a failed update leaves the old binary in place.
func (r *ReleaseService) Update(currentVersion string, binaryPath string) (*UpdateResult, error) {
	current, err := parseVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("drg %s was not installed from a release. Update it by building it from source again", currentVersion)
	}

	latestVersion, err := r.repo.LatestVersion()
	if err != nil {
		return nil, fmt.Errorf("could not look up the latest release: %w", err)
	}
	latest, err := parseVersion(latestVersion)
	if err != nil {
		return nil, fmt.Errorf("the latest release has an unexpected tag %q", latestVersion)
	}

	result := &UpdateResult{PreviousVersion: currentVersion, Version: latestVersion, BinaryPath: binaryPath}
	if compareVersions(current, latest) >= 0 {
		result.Version = currentVersion
		result.IsUpToDate = true
		return result, nil
	}

	archiveName := fmt.Sprintf(archiveNamePattern, runtime.GOOS, runtime.GOARCH)
	r.log.Info("Downloading %s (%s)", archiveName, latestVersion)
	archive, err := r.repo.DownloadFile(latestVersion, archiveName)
	if err != nil {
		return nil, fmt.Errorf("could not download %s of %s: %w", archiveName, latestVersion, err)
	}
	checksums, err := r.repo.DownloadFile(latestVersion, checksumsFileName)
	if err != nil {
		return nil, fmt.Errorf("could not download %s of %s: %w", checksumsFileName, latestVersion, err)
	}
	if err := verifyChecksum(archive, archiveName, checksums); err != nil {
		return nil, err
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", archiveName, err)
	}
	if err := replaceBinary(binaryPath, binary); err != nil {
		return nil, err
	}
	return result, nil
}

// ExecutablePath returns the path of the running drg binary with symlinks
// resolved. Update replaces the file a symlink points to and keeps the link.
func ExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not find the running drg binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve the drg binary at %s: %w", path, err)
	}
	return resolved, nil
}

func parseVersion(version string) ([versionNumberFields]int, error) {
	var numbers [versionNumberFields]int
	match := releaseVersionPattern.FindStringSubmatch(version)
	if match == nil {
		return numbers, fmt.Errorf("%q is not a release version", version)
	}
	for index := range numbers {
		number, err := strconv.Atoi(match[index+1])
		if err != nil {
			return numbers, fmt.Errorf("%q is not a release version: %w", version, err)
		}
		numbers[index] = number
	}
	return numbers, nil
}

func compareVersions(left, right [versionNumberFields]int) int {
	for index := range left {
		if left[index] != right[index] {
			return left[index] - right[index]
		}
	}
	return 0
}

// verifyChecksum checks archive against its line in a sha256sum style
// checksums file.
func verifyChecksum(archive []byte, archiveName string, checksums []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || fields[1] != archiveName {
			continue
		}
		sum := sha256.Sum256(archive)
		gotSum := hex.EncodeToString(sum[:])
		if gotSum != fields[0] {
			return fmt.Errorf("checksum of %s is %s, expected %s", archiveName, gotSum, fields[0])
		}
		return nil
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("could not read %s: %w", checksumsFileName, err)
	}
	return fmt.Errorf("%s has no entry for %s", checksumsFileName, archiveName)
}

func extractBinary(archive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the archive has no %s binary", binaryName)
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg && header.Name == binaryName {
			return io.ReadAll(tarReader)
		}
	}
}

// replaceBinary writes binary to a temp file beside binaryPath and renames it
// over binaryPath. A rename within one directory is atomic, and it works on a
// binary that is running.
func replaceBinary(binaryPath string, binary []byte) error {
	dir := filepath.Dir(binaryPath)
	tempFile, err := os.CreateTemp(dir, tempBinaryPattern)
	if err != nil {
		return fmt.Errorf("could not write to %s: %w. Run drg update as a user who can write there", dir, err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(binary); err != nil {
		tempFile.Close()
		return fmt.Errorf("could not write %s: %w", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("could not write %s: %w", tempPath, err)
	}
	if err := os.Chmod(tempPath, executableFilePerm); err != nil {
		return fmt.Errorf("could not make %s executable: %w", tempPath, err)
	}
	if err := os.Rename(tempPath, binaryPath); err != nil {
		return fmt.Errorf("could not replace %s: %w", binaryPath, err)
	}
	return nil
}
