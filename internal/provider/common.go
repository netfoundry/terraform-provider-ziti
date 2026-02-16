package provider

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
)

func doRequest(method, url, sessionToken string, body []byte) (string, error) {
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	req, _ := retryablehttp.NewRequest(method, url, bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %s, ERR: %v", url, err)
		return "", err
	}

	body, err = io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Error().Msgf("Error Creating Ziti Resource Response: %v", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode != http.StatusCreated {
			log.Printf("Unexpected status code: %d %s", resp.StatusCode, resp.Status)
			return string(body), errors.New(string(body))
		}
	}

	return string(body), nil
}

func CreateZitiResource(requestURL string, sessionToken string, payloadData []byte) (string, error) {
	return doRequest(http.MethodPost, requestURL, sessionToken, payloadData)
}

func ReadZitiResource(requestURL string, sessionToken string) (string, error) {
	return doRequest(http.MethodGet, requestURL, sessionToken, nil)
}

func UpdateZitiResource(requestURL string, sessionToken string, payloadData []byte) (string, error) {
	return doRequest(http.MethodPut, requestURL, sessionToken, payloadData)
}

func PatchZitiResource(requestURL string, sessionToken string, payloadData []byte) (string, error) {
	return doRequest(http.MethodPatch, requestURL, sessionToken, payloadData)
}

func DeleteZitiResource(requestURL string, sessionToken string) (string, error) {
	return doRequest(http.MethodDelete, requestURL, sessionToken, nil)
}
