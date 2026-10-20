// Command package-release builds the CLIProxyAPI plugin-store archives for one
// platform: one zip per protocol face, one bundle with all four faces, and a
// sha256sum checksums file. It uses only the Go standard library so the release
// workflow behaves the same on Linux, macOS and Windows.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var faces = []string{
	"cpa-opencodego",
	"cpa-opencodego-responses",
	"cpa-opencodego-messages",
	"cpa-opencodego-gemini",
}

func main() {
	version := flag.String("version", "", "release version without the leading v")
	goos := flag.String("goos", "", "target GOOS")
	goarch := flag.String("goarch", "", "target GOARCH")
	libdir := flag.String("libdir", "", "directory containing <face>-v<version>.<ext> libraries")
	outdir := flag.String("outdir", "", "directory for the generated archives")
	flag.Parse()

	if *version == "" || *goos == "" || *goarch == "" || *libdir == "" || *outdir == "" {
		fatalf("version, goos, goarch, libdir and outdir are required")
	}
	versionValue := strings.TrimPrefix(strings.TrimSpace(*version), "v")
	if versionValue == "" {
		fatalf("version is required")
	}

	extension := "so"
	switch *goos {
	case "darwin":
		extension = "dylib"
	case "windows":
		extension = "dll"
	}

	if errMkdir := os.MkdirAll(*outdir, 0o755); errMkdir != nil {
		fatalf("create output directory: %v", errMkdir)
	}

	libraries := make([]string, 0, len(faces))
	archives := make([]string, 0, len(faces)+1)
	for _, face := range faces {
		library := filepath.Join(*libdir, fmt.Sprintf("%s-v%s.%s", face, versionValue, extension))
		if _, errStat := os.Stat(library); errStat != nil {
			fatalf("missing library %s: %v", library, errStat)
		}
		libraries = append(libraries, library)
		archive := filepath.Join(*outdir, fmt.Sprintf("%s_%s_%s_%s.zip", face, versionValue, *goos, *goarch))
		if errWrite := writeZip(archive, []string{library}); errWrite != nil {
			fatalf("write %s: %v", archive, errWrite)
		}
		archives = append(archives, archive)
	}

	bundle := filepath.Join(*outdir, fmt.Sprintf("cpa-opencodego-bundle_%s_%s_%s.zip", versionValue, *goos, *goarch))
	if errWrite := writeZip(bundle, libraries); errWrite != nil {
		fatalf("write %s: %v", bundle, errWrite)
	}
	archives = append(archives, bundle)

	checksums := filepath.Join(*outdir, fmt.Sprintf("checksums-%s-%s.txt", *goos, *goarch))
	var lines strings.Builder
	for _, archive := range archives {
		digest, errDigest := sha256File(archive)
		if errDigest != nil {
			fatalf("checksum %s: %v", archive, errDigest)
		}
		fmt.Fprintf(&lines, "%s  %s\n", digest, filepath.Base(archive))
	}
	if errWrite := os.WriteFile(checksums, []byte(lines.String()), 0o644); errWrite != nil {
		fatalf("write checksums: %v", errWrite)
	}
	fmt.Printf("packaged %s/%s in %s\n", *goos, *goarch, *outdir)
}

func writeZip(archivePath string, files []string) error {
	file, errCreate := os.Create(archivePath)
	if errCreate != nil {
		return errCreate
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()

	writer := zip.NewWriter(file)
	// A fixed timestamp keeps the archives byte-for-byte reproducible.
	fixedTime := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, source := range files {
		data, errRead := os.ReadFile(source)
		if errRead != nil {
			return errRead
		}
		header := &zip.FileHeader{
			Name:     filepath.Base(source),
			Method:   zip.Deflate,
			Modified: fixedTime,
		}
		header.SetMode(0o755)
		entry, errCreateEntry := writer.CreateHeader(header)
		if errCreateEntry != nil {
			return errCreateEntry
		}
		if _, errCopy := entry.Write(data); errCopy != nil {
			return errCopy
		}
	}
	if errClose := writer.Close(); errClose != nil {
		return errClose
	}
	if errClose := file.Close(); errClose != nil {
		return errClose
	}
	closed = true
	return nil
}

func sha256File(path string) (string, error) {
	file, errOpen := os.Open(path)
	if errOpen != nil {
		return "", errOpen
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, errCopy := io.Copy(hash, file); errCopy != nil {
		return "", errCopy
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
