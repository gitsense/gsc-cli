/*
 * Component: Network Downloader
 * Block-UUID: 69b43ce5-c4c3-4f0f-811d-abcec622aa12
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides a utility function to download content from a URL into a temporary file.
 * Language: Go
 * Created-at: 2026-02-24T16:52:03.748Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package netutil

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// DownloadToTemp downloads the content from the given URL and stores it in a temporary file.
// It returns the handle to the temporary file, which is seeked to the beginning.
// The caller is responsible for closing and removing the file.
func DownloadToTemp(url string) (*os.File, error) {
	// 1. Create the HTTP Request
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate download from %s: %w", url, err)
	}
	defer resp.Body.Close()

	// 2. Check Response Status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status code %d for URL %s", resp.StatusCode, url)
	}

	// 3. Create Temporary File
	tmpFile, err := os.CreateTemp("", "gsc-manifest-download-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// 4. Stream Content to File
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write download content to temporary file: %w", err)
	}

	// 5. Rewind File Pointer for Reading
	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to rewind temporary file pointer: %w", err)
	}

	return tmpFile, nil
}
