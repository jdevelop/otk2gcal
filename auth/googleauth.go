package auth

type GoogleCalConf struct {
	Installed struct {
		AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
		AuthURI                 string   `json:"auth_uri"`
		ClientID                string   `json:"client_id"`
		ClientSecret            string   `json:"client_secret"`
		ProjectID               string   `json:"project_id"`
		RedirectUris            []string `json:"redirect_uris"`
		TokenURI                string   `json:"token_uri"`
	} `json:"installed"`
}
