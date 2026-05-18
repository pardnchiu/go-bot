package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}
	if len(body) > maxFileBytes {
		return "", fmt.Errorf("file too large: exceeds %d bytes (Size header was missing or wrong)", maxFileBytes)
	}

	name := go_pkg_utils.UUID()
	if name == "" {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}
	path := filepath.Join(dir, name+filepath.Ext(att.Filename))
	if err := go_pkg_filesystem.WriteFile(path, string(body), 0644); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile: %w", err)
	}
	return path, nil
}
