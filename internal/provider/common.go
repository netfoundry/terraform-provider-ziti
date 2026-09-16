package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
)

var errNotFound = errors.New("requested resource was not found")

// postRetryPolicy applies a stricter retry policy for POST requests, since
// POST is not idempotent: blindly retrying it can create the resource a
// second time if an earlier attempt actually reached the controller.
//
// GET/PUT/PATCH/DELETE are idempotent and keep using retryablehttp's
// DefaultRetryPolicy unchanged -- those retries were added for #34.
//
// For POST, only two situations are safe to retry:
//   - HTTP 429: the controller rejected the request outright, so nothing
//     was created. DefaultBackoff already honors Retry-After, so this stays
//     useful in rate-limited environments.
//   - Connection-establishment failures (DNS resolution, dial refused or
//     unreachable): the request provably never reached the controller.
//
// Everything else for POST -- 5xx, timeouts, mid-flight resets -- is
// ambiguous (the entity may already exist server-side) and is not retried.
func postRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// do not retry on context.Canceled or context.DeadlineExceeded
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if err != nil {
		return isConnectionEstablishmentError(err), nil
	}

	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	return false, nil
}

// isConnectionEstablishmentError reports whether err represents a failure to
// establish a connection at all -- DNS resolution failure, or a dial that
// was refused/unreachable -- as opposed to a failure that occurred after a
// connection was already established (read/write/reset), which is ambiguous
// about whether the request reached the controller.
func isConnectionEstablishmentError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial"
	}

	return false
}

// checkRetryPolicyFor returns the retryablehttp.CheckRetry policy to use for
// the given HTTP method. See postRetryPolicy for the POST-specific rules.
func checkRetryPolicyFor(method string) retryablehttp.CheckRetry {
	if method == http.MethodPost {
		return postRetryPolicy
	}
	return retryablehttp.DefaultRetryPolicy
}

func doRequest(method, url, sessionToken string, body []byte) (string, error) {
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient.CheckRetry = checkRetryPolicyFor(method)

	req, _ := retryablehttp.NewRequest(method, url, bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("zt-session", sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Msgf("Request failed on url: %s, ERR: %v", url, err)
		return "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", errNotFound
	}

	body, err = io.ReadAll(resp.Body)
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
