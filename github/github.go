package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/app/installations", api), nil)
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	installations := []AppInstallation{}
	if err := json.Unmarshal(body, &installations); err != nil {
		return nil, err
	}

	return installations, nil
}

func GetToken(api string, jwt string, appID int, body []byte) (*AppInstallationAccessToken, error) {
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

	tokenBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	token := AppInstallationAccessToken{}
	if err := json.Unmarshal(tokenBody, &token); err != nil {
		return nil, err
	}

	return &token, nil
}
