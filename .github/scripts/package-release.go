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
)

func main() {
	libraryPath := flag.String("library", "", "compiled shared library")
	archivePath := flag.String("archive", "", "output zip")
	checksumPath := flag.String("checksum", "", "output checksum")
	flag.Parse()
	if *libraryPath == "" || *archivePath == "" || *checksumPath == "" {
		fatalf("library, archive, and checksum are required")
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
