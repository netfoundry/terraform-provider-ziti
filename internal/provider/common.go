package provider

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func CreateZitiResource(authUrl string, sessionToken string, payloadData []byte) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	req, _ := http.NewRequest("POST", authUrl, bytes.NewBuffer(payloadData))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %s, ERR: %v", authUrl, err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
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

func ReadZitiResource(authUrl string, sessionToken string) (string, error) {
	const maxRetries = 3

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, _ := http.NewRequest("GET", authUrl, nil)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("zt-session", sessionToken)
		resp, err := httpClient.Do(req)

		if err != nil {
			log.Error().Msgf("Attempt %d: Got an error back from the Ziti controller: resourceType: %s, ERR: %v", attempt, authUrl, err)
			lastErr = err
		} else {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Error().Msgf("Error Reading Ziti Resource Response: %v", err)
				lastErr = err
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
		if attempt < maxRetries {
			time.Sleep(5 * time.Second)
		} else {
			log.Error().Msgf("request failed after %d retries, error: %v", attempt, lastErr)
		}
	}
	return "", lastErr
}

func UpdateZitiResource(authUrl string, sessionToken string, payloadData []byte) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	req, _ := http.NewRequest("PUT", authUrl, bytes.NewBuffer(payloadData))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %s, ERR: %v", authUrl, err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Error().Msgf("Error Updating Ziti Resource Response: %v", err)
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

func PatchZitiResource(authUrl string, sessionToken string, payloadData []byte) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	req, _ := http.NewRequest("PATCH", authUrl, bytes.NewBuffer(payloadData))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %s, ERR: %v", authUrl, err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Error().Msgf("Error Updating Ziti Resource Response: %v", err)
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

func DeleteZitiResource(authUrl string, sessionToken string) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	req, _ := http.NewRequest("DELETE", authUrl, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %s, ERR: %v", authUrl, err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Error().Msgf("Error Reading Ziti Resource Response: %v", err)
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
