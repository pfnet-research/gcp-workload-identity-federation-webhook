package webhooks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

/*

Adapted from:
https://github.com/golang/oauth2/blob/master/google/internal/externalaccount/basecredentials.go

*/

// Config stores the configuration for fetching tokens with external credentials.
type ExternalAccountCredentials struct {
	// Type is the Credentials file type - always 'external_account' in our case.
	Type string `json:"type"`
	// Audience is the Secure Token Service (STS) audience which contains the resource name for the workload
	// identity pool or the workforce pool and the provider identifier in that pool.
	Audience string `json:"audience"`
	// SubjectTokenType is the STS token type based on the Oauth2.0 token exchange spec
	// e.g. `urn:ietf:params:oauth:token-type:jwt`.
	SubjectTokenType string `json:"subject_token_type"`
	// TokenURL is the STS token exchange endpoint.
	TokenURL string `json:"token_url"`
	// TokenInfoURL is the token_info endpoint used to retrieve the account related information (
	// user attributes like account identifier, eg. email, username, uid, etc). This is
	// needed for gCloud session account identification.
	TokenInfoURL string `json:"token_info_url,omitempty"`
	// ServiceAccountImpersonationURL is the URL for the service account impersonation request. This is only
	// required for workload identity pools when APIs to be accessed have not integrated with UberMint.
	ServiceAccountImpersonationURL string `json:"service_account_impersonation_url"`
	// ServiceAccountImpersonationLifetimeSeconds is the number of seconds the service account impersonation
	// token will be valid for.
	ServiceAccountImpersonationLifetimeSeconds int `json:"service_account_impersonation_lifetime_seconds,omitempty"`
	// CredentialSource contains the necessary information to retrieve the token itself, as well
	// as some environmental information.
	CredentialSource CredentialSource `json:"credential_source"`
}

// CredentialSource stores the information necessary to retrieve the credentials for the STS exchange.
// One field amongst File, URL, and Executable should be filled, depending on the kind of credential in question.
// The EnvironmentID should start with AWS if being used for an AWS credential.
type CredentialSource struct {
	File   string           `json:"file"`
	Format CredentialFormat `json:"format"`
}

type CredentialFormat struct {
	// Type is either "text" or "json". When not provided "text" type is assumed.
	Type string `json:"type"`
}

func NewExternalAccountCredentials(aud, gsaEmail string) *ExternalAccountCredentials {
	creds := &ExternalAccountCredentials{
		Type:             "external_account",
		Audience:         aud,
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:         "https://sts.googleapis.com/v1/token",
		CredentialSource: CredentialSource{
			File:   filepath.Join(K8sSATokenMountPath, K8sSATokenName),
			Format: CredentialFormat{Type: "text"},
		},
		ServiceAccountImpersonationURL: fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken", gsaEmail),
	}

	return creds
}

// Render marshals the ExternalAccountCredentials object to a json string. Set indent = true to pretty print the
// json with indentation.
func (e *ExternalAccountCredentials) Render(indent bool) (string, error) {
	var b []byte
	var err error
	if indent {
		b, err = json.MarshalIndent(e, "", "  ")
	} else {
		b, err = json.Marshal(e)
	}
	if err != nil {
		return "", fmt.Errorf("could not marshal ExternalAccountCredentials to json: %w", err)
	}

	return string(b), nil
}
