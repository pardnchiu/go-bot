package line

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

const maxContentBytes = 50 << 20

func (b *Bot) Save(ctx context.Context, messageID, dir string) (string, error) {
	if messageID == "" {
		return "", fmt.Errorf("messageID is required")
	}
	if dir == "" {
		return "", fmt.Errorf("dir is required")
	}

	res, err := b.api.GetMessageContent(messageID).WithContext(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("line-bot-sdk-go GetMessageContent: %w", err)
	}
	defer res.Content.Close()
	if res.ContentLength > maxContentBytes {
		return "", fmt.Errorf("content too large: %d bytes (max %d)", res.ContentLength, maxContentBytes)
	}

	name := go_pkg_utils.UUID()
	if name == "" {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	finalPath := filepath.Join(dir, name+extFromContentType(res.ContentType))

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

	written, err := io.Copy(tmp, io.LimitReader(res.Content, maxContentBytes+1))
	if err != nil {
		cleanup()
		return "", fmt.Errorf("io.Copy: %w", err)
	}
	if written > maxContentBytes {
		cleanup()
		return "", fmt.Errorf("content too large: exceeds %d bytes (ContentLength was missing or wrong)", maxContentBytes)
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

func extFromContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	switch strings.TrimSpace(ct) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "audio/mp4", "audio/x-m4a":
		return ".m4a"
	case "audio/aac":
		return ".aac"
	case "application/pdf":
		return ".pdf"
	}
	if exts, err := mime.ExtensionsByType(ct); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}
