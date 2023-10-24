package webhooks

import "testing"

func TestExternalAccountCredentials_String(t *testing.T) {
	type fields struct {
		Audience string
		GSAEmail string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExternalAccountCredentials(tt.fields.Audience, tt.fields.GSAEmail)
			if got := e.String(); got != tt.want {
				t.Errorf("ExternalAccountCredentials.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
