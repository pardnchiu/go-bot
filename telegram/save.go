package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	tgBot "github.com/go-telegram/bot"
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

	name := go_pkg_utils.UUID()
	if name == "" {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	finalPath := filepath.Join(dir, name+filepath.Ext(f.FilePath))

	tmp, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("os.CreateTemp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		if err := tmp.Close(); err != nil {
			slog.Warn("tmp.Close (cleanup)",
				slog.String("tmpPath", tmpPath),
				slog.String("err", err.Error()))
		}
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("os.Remove tmp (cleanup)",
				slog.String("tmpPath", tmpPath),
				slog.String("err", err.Error()))
		}
	}

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxFileBytes+1))
	if err != nil {
		cleanup()
		return "", fmt.Errorf("io.Copy: %w", err)
	}
	if written > maxFileBytes {
		cleanup()
		return "", fmt.Errorf("file too large: exceeds %d bytes (FileSize header was missing or wrong)", maxFileBytes)
	}

	if err := tmp.Close(); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
			slog.Warn("os.Remove tmp (after tmp.Close fail)",
				slog.String("tmpPath", tmpPath),
				slog.String("err", rmErr.Error()))
		}
		return "", fmt.Errorf("tmp.Close: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
			slog.Warn("os.Remove tmp (after os.Rename fail)",
				slog.String("tmpPath", tmpPath),
				slog.String("err", rmErr.Error()))
		}
		return "", fmt.Errorf("os.Rename: %w", err)
	}
	return finalPath, nil
}
