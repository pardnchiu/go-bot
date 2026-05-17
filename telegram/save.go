package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	tgBot "github.com/go-telegram/bot"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

const maxFileBytes = 20 << 20

func (b *Bot) Save(ctx context.Context, fileID, dir string) (string, error) {
	if fileID == "" {
		return "", fmt.Errorf("fileID is required")
	}
	if dir == "" {
		return "", fmt.Errorf("dir is required")
	}

	f, err := b.api.GetFile(ctx, &tgBot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("go-telegram/bot Bot.GetFile: %w", err)
	}
	if f.FilePath == "" {
		return "", fmt.Errorf("Telegram returned empty file_path for fileID %s", fileID)
	}
	if f.FileSize > maxFileBytes {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", f.FileSize, maxFileBytes)
	}

	url := b.api.FileDownloadLink(f)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http.DefaultClient.Do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, raw)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}
	if int64(len(body)) > maxFileBytes {
		return "", fmt.Errorf("file too large: exceeds %d bytes (FileSize header was missing or wrong)", maxFileBytes)
	}

	name := go_pkg_utils.UUID()
	if name == "" {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}
	path := filepath.Join(dir, name+filepath.Ext(f.FilePath))
	if err := go_pkg_filesystem.WriteFile(path, string(body), 0644); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile: %w", err)
	}
	return path, nil
}
