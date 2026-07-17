package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	defaultModel  = "gemini-3.1-flash-tts-preview"
	defaultVoice  = "Kore"
	pcmSampleRate = "24000"
	opusBitrate   = "32k"
)

type response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func Get(ctx context.Context, apiKey, text string) ([]byte, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey is required")
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	pcm, err := call(ctx, apiKey, text)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	return pcmToOpus(ctx, pcm)
}

func call(ctx context.Context, apiKey, text string) ([]byte, error) {
	api := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", defaultModel, apiKey)

	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{
				{"text": text},
			}},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{
						"voiceName": defaultVoice,
					},
				},
			},
		},
	}

	resp, status, err := go_pkg_http.POST[response](ctx, nil, api, nil, body, "json")
	if err != nil {
		return nil, fmt.Errorf("POST status=%d: %w", status, err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		raw, _ := json.Marshal(resp)
		return nil, fmt.Errorf("empty response: %s", string(raw))
	}
	data := resp.Candidates[0].Content.Parts[0].InlineData.Data
	if data == "" {
		return nil, fmt.Errorf("no audio data")
	}
	return base64.StdEncoding.DecodeString(data)
}

func pcmToOpus(ctx context.Context, pcm []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "tts-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pcmPath := filepath.Join(tmpDir, "in.pcm")
	oggPath := filepath.Join(tmpDir, "out.ogg")

	if err := os.WriteFile(pcmPath, pcm, 0o600); err != nil {
		return nil, fmt.Errorf("write pcm: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", "-loglevel", "error",
		"-f", "s16le", "-ar", pcmSampleRate, "-ac", "1",
		"-i", pcmPath,
		"-c:a", "libopus", "-b:a", opusBitrate,
		oggPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w (stderr: %s)", err, string(out))
	}
	return os.ReadFile(oggPath)
}
