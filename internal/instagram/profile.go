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

type Profile struct {
	Username        string
	UserID          string
	ProfilePicURL   string
	ProfilePicURLHD string
	Media           []MediaItem
}

type MediaItem struct {
	URL       string
	IsVideo   bool
	Shortcode string
	TakenAt   int64
}

const (
	igAppID        = "936619743392459"
	defaultUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	profileInfoURL = "https://www.instagram.com/api/v1/users/web_profile_info/"
)

func FetchProfile(ctx context.Context, username string, cookies CookieBundle) (Profile, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return Profile{}, fmt.Errorf("username is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	payload, err := fetchProfilePayload(ctx, username, cookies, true)
	if err != nil {
		return Profile{}, err
	}
	user := payload.user
	if user == nil {
		return Profile{}, fmt.Errorf("no profile payload for %s", username)
	}
	return buildProfile(user), nil
}

type profilePayload struct {
	user *profileUser
}

type apiProfileResponse struct {
	Data *struct {
		User *profileUser `json:"user"`
	} `json:"data"`
	Graphql *struct {
		User *profileUser `json:"user"`
	} `json:"graphql"`
}

type profileUser struct {
	ID                       string         `json:"id"`
	PK                       string         `json:"pk"`
	Username                 string         `json:"username"`
	ProfilePicURL            string         `json:"profile_pic_url"`
	ProfilePicURLHD          string         `json:"profile_pic_url_hd"`
	EdgeOwnerToTimelineMedia mediaContainer `json:"edge_owner_to_timeline_media"`
}

type mediaContainer struct {
	Edges []mediaEdge `json:"edges"`
}

type mediaEdge struct {
	Node mediaNode `json:"node"`
}

type mediaNode struct {
	DisplayURL       string `json:"display_url"`
	ThumbnailSrc     string `json:"thumbnail_src"`
	IsVideo          bool   `json:"is_video"`
	Shortcode        string `json:"shortcode"`
	TakenAtTimestamp int64  `json:"taken_at_timestamp"`
}

func fetchProfilePayload(
	ctx context.Context,
	username string,
	cookies CookieBundle,
	allowFallback bool,
) (profilePayload, error) {
	query := url.Values{}
	query.Set("username", username)
	apiURL := profileInfoURL + "?" + query.Encode()

	body, status, err := doJSONRequest(ctx, apiURL, username, cookies)
	if err == nil && status == http.StatusOK {
		payload, err := decodeProfile(body)
		if err == nil && payload.user != nil {
			return payload, nil
		}
		if err == nil {
			return payload, fmt.Errorf("profile payload missing user")
		}
	}

	if !allowFallback {
		return profilePayload{}, fmt.Errorf("profile fetch failed (%d): %s", status, errText(err))
	}

	fallbackURL := fmt.Sprintf("https://www.instagram.com/%s/?__a=1&__d=dis", url.PathEscape(username))
	body, status, err = doJSONRequest(ctx, fallbackURL, username, cookies)
	if err != nil {
		return profilePayload{}, fmt.Errorf("profile fetch failed (%d): %s", status, errText(err))
	}
	payload, err := decodeProfile(body)
	if err != nil {
		return profilePayload{}, err
	}
	if payload.user == nil {
		return profilePayload{}, fmt.Errorf("profile payload missing user")
	}
	return payload, nil
}

func doJSONRequest(
	ctx context.Context,
	endpoint string,
	username string,
	cookies CookieBundle,
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, resp.StatusCode, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

func decodeProfile(body []byte) (profilePayload, error) {
	var raw apiProfileResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return profilePayload{}, err
	}
	if raw.Data != nil && raw.Data.User != nil {
		return profilePayload{user: raw.Data.User}, nil
	}
	if raw.Graphql != nil && raw.Graphql.User != nil {
		return profilePayload{user: raw.Graphql.User}, nil
	}
	return profilePayload{}, nil
}

func applyHeaders(req *http.Request, username string, cookies CookieBundle) {
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("X-IG-App-ID", igAppID)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if cookies.Header != "" {
		req.Header.Set("Cookie", cookies.Header)
	}
	if cookies.CSRFToken != "" {
		req.Header.Set("X-CSRFToken", cookies.CSRFToken)
	}
	if strings.TrimSpace(username) != "" {
		req.Header.Set("Referer", fmt.Sprintf("https://www.instagram.com/%s/", username))
	}
}

func buildProfile(user *profileUser) Profile {
	userID := strings.TrimSpace(user.ID)
	if userID == "" {
		userID = strings.TrimSpace(user.PK)
	}
	profile := Profile{
		Username:        user.Username,
		UserID:          userID,
		ProfilePicURL:   user.ProfilePicURL,
		ProfilePicURLHD: user.ProfilePicURLHD,
	}

	for _, edge := range user.EdgeOwnerToTimelineMedia.Edges {
		node := edge.Node
		url := strings.TrimSpace(node.DisplayURL)
		if node.IsVideo && node.ThumbnailSrc != "" {
			url = strings.TrimSpace(node.ThumbnailSrc)
		}
		if url == "" {
			continue
		}
		profile.Media = append(profile.Media, MediaItem{
			URL:       url,
			IsVideo:   node.IsVideo,
			Shortcode: node.Shortcode,
			TakenAt:   node.TakenAtTimestamp,
		})
	}

	return profile
}

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}
