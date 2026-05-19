package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

const maxFileBytes = 25 << 20

func (b *Bot) Save(ctx context.Context, att *discordgo.MessageAttachment, dir string) (string, error) {
	if att == nil {
		return "", fmt.Errorf("attachment is required")
	}
	if att.URL == "" {
		return "", fmt.Errorf("attachment URL is empty")
	}
	if dir == "" {
		return "", fmt.Errorf("dir is required")
	}
	if att.Size > maxFileBytes {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", att.Size, maxFileBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, att.URL, nil)
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

	finalPath := filepath.Join(dir, name+filepath.Ext(att.Filename))

	tmp, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("os.CreateTemp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxFileBytes+1))
	if err != nil {
		cleanup()
		return "", fmt.Errorf("io.Copy: %w", err)
	}
	if written > maxFileBytes {
		cleanup()
		return "", fmt.Errorf("file too large: exceeds %d bytes (Size header was missing or wrong)", maxFileBytes)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("tmp.Close: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("os.Rename: %w", err)
	}
	return finalPath, nil
}
