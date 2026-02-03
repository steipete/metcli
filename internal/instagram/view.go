package instagram

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

type Item struct {
	URL       string
	Kind      string
	IsVideo   bool
	Shortcode string
	TakenAt   int64
}

func ParseUsername(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if strings.HasPrefix(input, "@") {
		return strings.TrimPrefix(input, "@")
	}
	if !strings.Contains(input, "instagram.com") {
		return input
	}
	parsed, err := url.Parse(input)
	if err != nil {
		return input
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return ""
	}
	return segments[0]
}

func BuildItems(profile Profile, includeAvatar bool, includeVideos bool) []Item {
	items := make([]Item, 0, len(profile.Media)+1)
	if includeAvatar {
		avatarURL := strings.TrimSpace(profile.ProfilePicURLHD)
		if avatarURL == "" {
			avatarURL = strings.TrimSpace(profile.ProfilePicURL)
		}
		if avatarURL != "" {
			items = append(items, Item{
				URL:  avatarURL,
				Kind: "avatar",
			})
		}
	}
	for _, media := range profile.Media {
		if media.URL == "" {
			continue
		}
		if media.IsVideo && !includeVideos {
			continue
		}
		items = append(items, Item{
			URL:       media.URL,
			Kind:      "media",
			IsVideo:   media.IsVideo,
			Shortcode: media.Shortcode,
			TakenAt:   media.TakenAt,
		})
	}
	return items
}

func InlineName(item Item) string {
	base := item.Shortcode
	if base == "" {
		base = "instagram"
	}
	return path.Base(base + ".img")
}

func DownloadImage(
	ctx context.Context,
	client *http.Client,
	imgURL string,
	username string,
	cookies CookieBundle,
) ([]byte, int, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "image/jpeg,image/png,image/*;q=0.8,*/*;q=0.5")
	if cookies.Header != "" {
		req.Header.Set("Cookie", cookies.Header)
	}
	if strings.TrimSpace(username) != "" {
		req.Header.Set("Referer", fmt.Sprintf("https://www.instagram.com/%s/", username))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, 0, fmt.Errorf("fetch %s: %d %s", imgURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 15<<20))
	if err != nil {
		return nil, 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return data, 0, 0, nil
	}
	return data, cfg.Width, cfg.Height, nil
}

func EnsurePNG(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func ImageClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}
