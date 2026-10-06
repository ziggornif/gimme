package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
)

// Options describes a package upload.
type Options struct {
	URL, Token, Name, Version, ArchivePath string
}

// Push uploads an archive to a gimme server.
func Push(ctx context.Context, client *http.Client, opts Options) error {
	bodyReader, bodyWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(bodyWriter)
	go func() {
		err := writeMultipart(multipartWriter, opts)
		if err == nil {
			err = multipartWriter.Close()
		}
		_ = bodyWriter.CloseWithError(err)
	}()

	endpoint := strings.TrimRight(opts.URL, "/") + "/packages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload package: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 513))
	if readErr != nil {
		return fmt.Errorf("read server response: %w", readErr)
	}
	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed (401): the token is missing, invalid, revoked or expired")
	}
	message, jsonBody := serverMessage(body)
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("409 Conflict: %s; versions are immutable, publish a new version", message)
	}
	if resp.StatusCode == http.StatusRequestEntityTooLarge && !jsonBody {
		return fmt.Errorf("413 Request Entity Too Large: upload is too large for the server or a reverse proxy in front of it; check upload.max_size and nginx client_max_body_size")
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if len(body) == 513 && !jsonBody {
		message = strings.TrimSpace(string(body[:512])) + "…"
	}
	if message == "" {
		return fmt.Errorf("%s", resp.Status)
	}
	return fmt.Errorf("%s: %s", resp.Status, message)
}

func writeMultipart(writer *multipart.Writer, opts Options) error {
	if err := writer.WriteField("name", opts.Name); err != nil {
		return fmt.Errorf("write name field: %w", err)
	}
	if err := writer.WriteField("version", opts.Version); err != nil {
		return fmt.Errorf("write version field: %w", err)
	}
	file, err := os.Open(opts.ArchivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = file.Close() }()
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="package.zip"`)
	header.Set("Content-Type", "application/zip")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create archive part: %w", err)
	}
	if _, err = io.Copy(part, file); err != nil {
		return fmt.Errorf("write archive part: %w", err)
	}
	return nil
}

func serverMessage(body []byte) (string, bool) {
	var response struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil && response.Error != "" {
		return response.Error, true
	}
	return "", false
}
