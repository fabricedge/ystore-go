package formats

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ArchiveMagic = [4]byte{0x59, 0x53, 0x54, 0x47}

type BundledFile struct {
	Name string
	Data []byte
}

func BundleFiles(files []BundledFile) ([]byte, error) {
	var buf bytes.Buffer

	header := make([]byte, 8)
	copy(header[0:4], ArchiveMagic[:])
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(files)))
	buf.Write(header)

	tw := tar.NewWriter(&buf)

	for _, f := range files {
		hdr := &tar.Header{
			Name:     filepath.ToSlash(f.Name),
			Mode:     0644,
			Size:     int64(len(f.Data)),
			ModTime:  time.Now(),
			Format:   tar.FormatUSTAR,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("tar header %s: %w", f.Name, err)
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, fmt.Errorf("tar write %s: %w", f.Name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}

	return buf.Bytes(), nil
}

func IsArchive(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.Equal(data[:4], ArchiveMagic[:])
}

func ExtractFiles(data []byte) ([]BundledFile, error) {
	if !IsArchive(data) {
		return nil, fmt.Errorf("not an archive: missing magic")
	}

	numFiles := binary.LittleEndian.Uint32(data[4:8])
	tarData := data[8:]

	tr := tar.NewReader(bytes.NewReader(tarData))
	var files []BundledFile

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, fmt.Errorf("tar copy %s: %w", hdr.Name, err)
		}

		files = append(files, BundledFile{
			Name: hdr.Name,
			Data: buf.Bytes(),
		})
	}

	if len(files) != int(numFiles) {
		return nil, fmt.Errorf("expected %d files in archive, got %d", numFiles, len(files))
	}

	return files, nil
}

func ReadFilesFromDisk(paths []string) ([]BundledFile, error) {
	var files []BundledFile

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		files = append(files, BundledFile{
			Name: filepath.Base(path),
			Data: data,
		})
	}

	return files, nil
}

func ReadDir(path string, patterns ...string) ([]BundledFile, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading dir %s: %w", path, err)
	}

	var files []BundledFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fullPath := filepath.Join(path, e.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", fullPath, err)
		}
		files = append(files, BundledFile{
			Name: e.Name(),
			Data: data,
		})
	}

	return files, nil
}
