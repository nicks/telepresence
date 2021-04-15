package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/telepresenceio/telepresence/v2/pkg/client"
)

type LicenseInfo struct {
	ID             string      `json:"id"`             // License ID, used to identify the license
	Description    string      `json:"description"`    // Description for which the license should be used
	Audiences      []string    `json:"audiences"`      // Cluster IDs that this license can be used with
	ExpirationDate string      `json:"expirationDate"` // ($date-time) the license will expire on
	Limits         interface{} `json:"limits"`         // Map of limits determining bound to the license
}

// getLicenseJWT does the REST call to system and returns the jwt formatted license on success
func getLicenseJWT(ctx context.Context, env client.Env, accessToken, licenseID string) (string, error) {
	// Build the request.
	req, err := http.NewRequestWithContext(ctx,
		http.MethodGet, fmt.Sprintf("https://%s/api/licenses/%s/formats/jwt", env.LoginDomain, licenseID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send the request.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// From here on out, we should wrap any errors to include the HTTP response status code.

	// Sanity-check the content type before reading the body.
	mimetype, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("http %v: %w", resp.StatusCode, err)
	}
	if mimetype != "text/plain" {
		return "", fmt.Errorf("http %v: response body is not text/plain: %q", resp.StatusCode, mimetype)
	}

	// Read the body.
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// No need to wrap this error with the status code, the magical resp.Body reader
		// includes it for us.
		return "", err
	}
	// Output is different depending on the status code so we return custom errors depending
	// on the status of the request
	switch resp.StatusCode {
	case 404:
		return "", fmt.Errorf("license JWT not found")
	case 500:
		return "", fmt.Errorf("Error getting license jwt for %s: %s", licenseID, string(bodyBytes))
	case 200:
	}
	return string(bodyBytes), nil
}

// GetLicense is added as part of the loginExecutor so it can utilize the
// access token to talk to systemA in getLicenseJWT
func (l *loginExecutor) GetLicense(ctx context.Context, id string) (string, error) {
	l.loginMu.Lock()
	defer l.loginMu.Unlock()
	if l.tokenSource == nil {
		return "", fmt.Errorf("GetLicense: %w", ErrNotLoggedIn)
	} else if tokenInfo, err := l.tokenSource.Token(); err != nil {
		return "", err
	} else if license, err := getLicenseJWT(ctx, l.env, tokenInfo.AccessToken, id); err != nil {
		return "", err
	} else {
		if license == "" {
			return "", fmt.Errorf("No licenses found for %s", id)
		}
		return license, nil
	}
}
