package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type feedResponse struct {
	Items         []feedItem `json:"items"`
	MoreAvailable bool       `json:"more_available"`
	NextMaxID     string     `json:"next_max_id"`
}

type feedItem struct {
	MediaType     int             `json:"media_type"`
	ImageVersions imageVersions   `json:"image_versions2"`
	CarouselMedia []carouselMedia `json:"carousel_media"`
	ThumbnailURL  string          `json:"thumbnail_url"`
	Code          string          `json:"code"`
	Shortcode     string          `json:"shortcode"`
	TakenAt       int64           `json:"taken_at"`
}

type carouselMedia struct {
	MediaType     int           `json:"media_type"`
	ImageVersions imageVersions `json:"image_versions2"`
	ThumbnailURL  string        `json:"thumbnail_url"`
}

type imageVersions struct {
	Candidates []imageCandidate `json:"candidates"`
}

type imageCandidate struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func FetchUserMedia(
	ctx context.Context,
	username string,
	profile Profile,
	cookies CookieBundle,
	max int,
	pageSize int,
) ([]MediaItem, error) {
	if max == 0 {
		max = -1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 50 {
		pageSize = 50
	}

	out := make([]MediaItem, 0, len(profile.Media))
	seen := map[string]struct{}{}
	appendUnique := func(items []MediaItem) {
		for _, item := range items {
			if item.URL == "" {
				continue
			}
			if _, ok := seen[item.URL]; ok {
				continue
			}
			seen[item.URL] = struct{}{}
			out = append(out, item)
		}
	}

	appendUnique(profile.Media)
	if max > 0 && len(out) >= max {
		return out[:max], nil
	}

	userID := strings.TrimSpace(profile.UserID)
	if userID == "" {
		return out, nil
	}

	maxID := ""
	pageCount := 0
	for {
		pageCount++
		page, err := fetchUserFeedPage(ctx, username, userID, maxID, pageSize, cookies)
		if err != nil {
			return out, err
		}
		appendUnique(page.items)
		if max > 0 && len(out) >= max {
			return out[:max], nil
		}
		if !page.moreAvailable || page.nextMaxID == "" {
			break
		}
		if page.nextMaxID == maxID {
			break
		}
		maxID = page.nextMaxID
		if pageCount > 200 {
			break
		}
	}

	return out, nil
}

type feedPage struct {
	items         []MediaItem
	moreAvailable bool
	nextMaxID     string
}

func fetchUserFeedPage(
	ctx context.Context,
	username string,
	userID string,
	maxID string,
	pageSize int,
	cookies CookieBundle,
) (feedPage, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 50 {
		pageSize = 50
	}
	endpoint := fmt.Sprintf(
		"https://www.instagram.com/api/v1/feed/user/%s/?count=%d",
		url.PathEscape(userID),
		pageSize,
	)
	if strings.TrimSpace(maxID) != "" {
		endpoint += "&max_id=" + url.QueryEscape(maxID)
	}

	body, status, err := doJSONRequestWithLimit(ctx, endpoint, username, cookies, 4<<20)
	if err != nil {
		return feedPage{}, fmt.Errorf("feed request failed (%d): %s", status, errText(err))
	}

	var raw feedResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return feedPage{}, err
	}

	items := make([]MediaItem, 0, len(raw.Items))
	for _, item := range raw.Items {
		items = append(items, feedItemToMedia(item)...)
	}

	return feedPage{
		items:         items,
		moreAvailable: raw.MoreAvailable,
		nextMaxID:     raw.NextMaxID,
	}, nil
}

func feedItemToMedia(item feedItem) []MediaItem {
	shortcode := item.Code
	if shortcode == "" {
		shortcode = item.Shortcode
	}

	switch item.MediaType {
	case 8:
		return expandCarousel(item, shortcode)
	case 2:
		url := strings.TrimSpace(item.ThumbnailURL)
		if url == "" {
			url = pickBestCandidate(item.ImageVersions.Candidates)
		}
		if url == "" {
			return nil
		}
		return []MediaItem{{
			URL:       url,
			IsVideo:   true,
			Shortcode: shortcode,
			TakenAt:   item.TakenAt,
		}}
	default:
		url := pickBestCandidate(item.ImageVersions.Candidates)
		if url == "" {
			return nil
		}
		return []MediaItem{{
			URL:       url,
			IsVideo:   false,
			Shortcode: shortcode,
			TakenAt:   item.TakenAt,
		}}
	}
}

func expandCarousel(item feedItem, shortcode string) []MediaItem {
	items := make([]MediaItem, 0, len(item.CarouselMedia))
	for _, media := range item.CarouselMedia {
		isVideo := media.MediaType == 2
		url := ""
		if isVideo {
			url = strings.TrimSpace(media.ThumbnailURL)
		}
		if url == "" {
			url = pickBestCandidate(media.ImageVersions.Candidates)
		}
		if url == "" {
			continue
		}
		items = append(items, MediaItem{
			URL:       url,
			IsVideo:   isVideo,
			Shortcode: shortcode,
			TakenAt:   item.TakenAt,
		})
	}
	return items
}

func pickBestCandidate(candidates []imageCandidate) string {
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	bestScore := best.Width * best.Height
	for _, candidate := range candidates[1:] {
		score := candidate.Width * candidate.Height
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return strings.TrimSpace(best.URL)
}

func doJSONRequestWithLimit(
	ctx context.Context,
	endpoint string,
	username string,
	cookies CookieBundle,
	limit int64,
) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	applyHeaders(req, username, cookies)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := readLimited(resp, limit)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, resp.StatusCode, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

func readLimited(resp *http.Response, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = 2 << 20
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}
