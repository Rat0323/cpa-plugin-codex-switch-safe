package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type artifactManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"version"`
	GOOS          string `json:"goos"`
	GOARCH        string `json:"goarch"`
	SourceCommit  string `json:"source_commit"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	Library       string `json:"library"`
	LibrarySize   int64  `json:"library_size"`
	LibrarySHA256 string `json:"library_sha256"`
}

func main() {
	libraryPath := flag.String("library", "", "compiled shared library")
	archivePath := flag.String("archive", "", "output zip")
	checksumPath := flag.String("checksum", "", "output checksum")
	manifestPath := flag.String("manifest", "", "output artifact manifest")
	version := flag.String("version", "", "plugin version")
	goos := flag.String("goos", "", "target operating system")
	goarch := flag.String("goarch", "", "target architecture")
	sourceCommit := flag.String("source-commit", "", "source commit hash")
	flag.Parse()
	if *libraryPath == "" || *archivePath == "" || *checksumPath == "" || *manifestPath == "" ||
		*version == "" || *goos == "" || *goarch == "" || *sourceCommit == "" {
		fatalf("library, archive, checksum, manifest, version, goos, goarch, and source-commit are required")
	}
	data, errPackage := packageLibrary(*libraryPath, *archivePath)
	if errPackage != nil {
		fatalf("package: %v", errPackage)
	}
	sum := sha256.Sum256(data)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), filepath.Base(*archivePath))
	if errWrite := os.WriteFile(*checksumPath, []byte(line), 0o644); errWrite != nil {
		fatalf("checksum: %v", errWrite)
	}
	manifest, errManifest := buildArtifactManifest(*libraryPath, *archivePath, data, *version, *goos, *goarch, *sourceCommit)
	if errManifest != nil {
		fatalf("manifest: %v", errManifest)
	}
	manifestJSON, errMarshal := json.MarshalIndent(manifest, "", "  ")
	if errMarshal != nil {
		fatalf("manifest: %v", errMarshal)
	}
	manifestJSON = append(manifestJSON, '\n')
	if errWrite := os.WriteFile(*manifestPath, manifestJSON, 0o644); errWrite != nil {
		fatalf("manifest: %v", errWrite)
	}
}

func buildArtifactManifest(libraryPath, archivePath string, archiveData []byte, version, goos, goarch, sourceCommit string) (artifactManifest, error) {
	reader, errOpen := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if errOpen != nil {
		return artifactManifest{}, errOpen
	}
	if len(reader.File) != 1 || reader.File[0].Name != filepath.Base(libraryPath) {
		return artifactManifest{}, fmt.Errorf("archive must contain only %s", filepath.Base(libraryPath))
	}
	library, errLibraryOpen := reader.File[0].Open()
	if errLibraryOpen != nil {
		return artifactManifest{}, errLibraryOpen
	}
	libraryData, errRead := io.ReadAll(library)
	errClose := library.Close()
	if errRead != nil {
		return artifactManifest{}, errRead
	}
	if errClose != nil {
		return artifactManifest{}, errClose
	}
	archiveSum := sha256.Sum256(archiveData)
	librarySum := sha256.Sum256(libraryData)
	return artifactManifest{
		SchemaVersion: 1,
		Version:       version,
		GOOS:          goos,
		GOARCH:        goarch,
		SourceCommit:  sourceCommit,
		Archive:       filepath.Base(archivePath),
		ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
		Library:       reader.File[0].Name,
		LibrarySize:   int64(len(libraryData)),
		LibrarySHA256: hex.EncodeToString(librarySum[:]),
	}, nil
}

func packageLibrary(libraryPath, archivePath string) ([]byte, error) {
	library, errOpen := os.Open(libraryPath)
	if errOpen != nil {
		return nil, errOpen
	}
	defer library.Close()
	info, errStat := library.Stat()
	if errStat != nil {
		return nil, errStat
	}
	archive, errCreate := os.Create(archivePath)
	if errCreate != nil {
		return nil, errCreate
	}
	writer := zip.NewWriter(archive)
	header, errHeader := zip.FileInfoHeader(info)
	if errHeader != nil {
		archive.Close()
		return nil, errHeader
	}
	header.Name = filepath.Base(libraryPath)
	header.Method = zip.Deflate
	header.SetMode(0o755)
	entry, errEntry := writer.CreateHeader(header)
	if errEntry != nil {
		archive.Close()
		return nil, errEntry
	}
	if _, errCopy := io.Copy(entry, library); errCopy != nil {
		archive.Close()
		return nil, errCopy
	}
	if errClose := writer.Close(); errClose != nil {
		archive.Close()
		return nil, errClose
	}
	if errClose := archive.Close(); errClose != nil {
		return nil, errClose
	}
	return os.ReadFile(archivePath)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
