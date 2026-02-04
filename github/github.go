package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/plumber-cd/github-apps-trampoline/logger"
)

type AppInstallationAccount struct {
	Login string `json:"login"`
}

type AppInstallation struct {
	ID      int                    `json:"id"`
	Account AppInstallationAccount `json:"account"`
}

type AppInstallationAccessToken struct {
	Token string `json:"token"`
}

func GetInstallations(api string, jwt string) ([]AppInstallation, error) {
	logger.Get().Printf("Getting known installations for jwt from %s", api)

	client := http.Client{}
	installations := []AppInstallation{}
	page := 1
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/app/installations?per_page=100&page=%d", api, page), nil)
		if err != nil {
			return nil, err
		}

		req.Header = http.Header{
			"Accept":        []string{"application/vnd.github.v3+json"},
			"Authorization": []string{"Bearer " + jwt},
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := readBody(resp)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("failed to list installations: status=%d body=%s", resp.StatusCode, body)
		}

		pageInstallations := []AppInstallation{}
		if err := json.Unmarshal([]byte(body), &pageInstallations); err != nil {
			return nil, err
		}

		logger.Get().Printf("Found page %d: %d installations", page, len(pageInstallations))
		installations = append(installations, pageInstallations...)

		if !hasNextPage(resp.Header.Get("Link")) || len(pageInstallations) == 0 {
			break
		}
		page += 1
	}

	return installations, nil
}

func GetToken(api string, jwt string, appID int, body []byte) (*AppInstallationAccessToken, error) {
	logger.Get().Printf("Getting token for appID=%d with current jwt from %s: %s", appID, api, string(body))

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/app/installations/%d/access_tokens", api, appID), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header = http.Header{
		"Accept":        []string{"application/vnd.github.v3+json"},
		"Authorization": []string{"Bearer " + jwt},
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	tokenBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get token: status=%d body=%s", resp.StatusCode, tokenBody)
	}

	logger.Get().Printf("Token response: %s", tokenBody)

	token := AppInstallationAccessToken{}
	if err := json.Unmarshal([]byte(tokenBody), &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func readBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	body := string(raw)
	if len(body) > 4096 {
		body = body[:4096] + "...(truncated)"
	}
	return body, nil
}

func hasNextPage(linkHeader string) bool {
	if linkHeader == "" {
		return false
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		if strings.Contains(part, `rel="next"`) {
			return true
		}
	}
	return false
}
