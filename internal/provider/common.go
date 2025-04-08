package provider

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"

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
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %v, Status Code: %v, ERR: %v", authUrl, resp.StatusCode, err)
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
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	req, _ := http.NewRequest("GET", authUrl, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %v, Status Code: %v, ERR: %v", authUrl, resp.StatusCode, err)
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
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %v, Status Code: %v, ERR: %v", authUrl, resp.StatusCode, err)
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
		log.Error().Msgf("Got an error back from the Ziti controller: resourceType: %v, Status Code: %v, ERR: %v", authUrl, resp.StatusCode, err)
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
