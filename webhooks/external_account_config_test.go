package webhooks

import "testing"

func TestExternalAccountCredentials_Render(t *testing.T) {
	type fields struct {
		Audience string
		GSAEmail string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			fields: fields{
				Audience: "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/workload-identity-pool/providers/workload-identity",
				GSAEmail: "workload@PROJECT.iam.gserviceaccount.com",
			},
			want: `{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/workload-identity-pool/providers/workload-identity",
  "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
  "token_url": "https://sts.googleapis.com/v1/token",
  "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/workload@PROJECT.iam.gserviceaccount.com:generateAccessToken",
  "credential_source": {
    "file": "/var/run/secrets/sts.googleapis.com/serviceaccount/token",
    "format": {
      "type": "text"
    }
  }
}`,
		},
		{
			name: "direct-access (no impersonation)",
			fields: fields{
				Audience: "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/workload-identity-pool/providers/workload-identity",
				GSAEmail: "",
			},
			want: `{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/workload-identity-pool/providers/workload-identity",
  "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
  "token_url": "https://sts.googleapis.com/v1/token",
  "credential_source": {
    "file": "/var/run/secrets/sts.googleapis.com/serviceaccount/token",
    "format": {
      "type": "text"
    }
  }
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var emailArg *string
			if tt.fields.GSAEmail != "" {
				e := tt.fields.GSAEmail
				emailArg = &e
			}
			e := NewExternalAccountCredentials(tt.fields.Audience, emailArg)
			got, err := e.Render(true)
			if err != nil && !tt.wantErr {
				t.Errorf("ExternalAccountCredentials.Render() returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ExternalAccountCredentials.Render() = %v, want %v", got, tt.want)
			}
		})
	}
}
