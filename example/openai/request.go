package openai

import (
	"net/http"
)

func NewRequest(url string, apiKey string) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	return req, nil
}
