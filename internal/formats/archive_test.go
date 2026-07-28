package formats

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleAndExtractSingleFile(t *testing.T) {
	files := []BundledFile{
		{Name: "hello.txt", Data: []byte("Hello, World!")},
	}

	bundle, err := BundleFiles(files)
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}

	if !IsArchive(bundle) {
		t.Fatal("IsArchive returned false for valid archive")
	}

	extracted, err := ExtractFiles(bundle)
	if err != nil {
		t.Fatalf("ExtractFiles: %v", err)
	}

	if len(extracted) != 1 {
		t.Fatalf("expected 1 file, got %d", len(extracted))
	}
	if extracted[0].Name != "hello.txt" {
		t.Errorf("expected name hello.txt, got %s", extracted[0].Name)
	}
	if string(extracted[0].Data) != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", string(extracted[0].Data))
	}
}

func TestBundleAndExtractMultipleFiles(t *testing.T) {
	files := []BundledFile{
		{Name: "a.txt", Data: []byte("AAA")},
		{Name: "b.txt", Data: []byte("BBB")},
		{Name: "sub/c.txt", Data: []byte("CCC")},
	}

	bundle, err := BundleFiles(files)
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}

	extracted, err := ExtractFiles(bundle)
	if err != nil {
		t.Fatalf("ExtractFiles: %v", err)
	}

	if len(extracted) != 3 {
		t.Fatalf("expected 3 files, got %d", len(extracted))
	}

	byName := make(map[string]string)
	for _, f := range extracted {
		byName[f.Name] = string(f.Data)
	}

	if byName["a.txt"] != "AAA" {
		t.Errorf("a.txt data mismatch")
	}
	if byName["b.txt"] != "BBB" {
		t.Errorf("b.txt data mismatch")
	}
	if byName["sub/c.txt"] != "CCC" {
		t.Errorf("sub/c.txt data mismatch")
	}
}

func TestIsArchiveFalse(t *testing.T) {
	if IsArchive([]byte{0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatal("IsArchive should be false for zero bytes")
	}
	if IsArchive([]byte{}) {
		t.Fatal("IsArchive should be false for empty data")
	}
	if IsArchive(nil) {
		t.Fatal("IsArchive should be false for nil")
	}
}

func TestExtractInvalidData(t *testing.T) {
	_, err := ExtractFiles([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	if err == nil {
		t.Fatal("expected error for invalid archive")
	}
}

func TestReadFilesFromDisk(t *testing.T) {
	dir := t.TempDir()

	paths := []string{
		filepath.Join(dir, "f1.txt"),
		filepath.Join(dir, "f2.txt"),
	}

	if err := os.WriteFile(paths[0], []byte("file1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[1], []byte("file2"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ReadFilesFromDisk(paths)
	if err != nil {
		t.Fatalf("ReadFilesFromDisk: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	byName := make(map[string]string)
	for _, f := range files {
		byName[f.Name] = string(f.Data)
	}

	if byName["f1.txt"] != "file1" {
		t.Errorf("f1.txt mismatch")
	}
	if byName["f2.txt"] != "file2" {
		t.Errorf("f2.txt mismatch")
	}
}

func TestReadDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.dat"), []byte("111"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.dat"), []byte("222"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	files, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files (excluding dir), got %d", len(files))
	}
}

func TestBundleRoundTripLargeFile(t *testing.T) {
	largeData := bytes.Repeat([]byte("ABCDEFGH"), 10000)
	files := []BundledFile{
		{Name: "large.bin", Data: largeData},
	}

	bundle, err := BundleFiles(files)
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}

	extracted, err := ExtractFiles(bundle)
	if err != nil {
		t.Fatalf("ExtractFiles: %v", err)
	}

	if len(extracted) != 1 {
		t.Fatalf("expected 1 file, got %d", len(extracted))
	}
	if !bytes.Equal(extracted[0].Data, largeData) {
		t.Errorf("data mismatch for large file")
	}
}
