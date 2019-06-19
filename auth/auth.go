package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	scopes = "Calendars.ReadWrite User.Read offline_access openid"
)

type Auth struct {
	clientId     string
	clientSecret string
	callback     string
	c            *http.Client
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func New(clientId, clientSecret, oauthCallbackUrl string, client *http.Client) (*Auth, error) {
	return &Auth{
		clientId:     clientId,
		callback:     oauthCallbackUrl,
		clientSecret: clientSecret,
		c:            client,
	}, nil
}

func (a *Auth) GetTokens() (*Tokens, error) {
	u, err := url.Parse(a.callback)
	if err != nil {
		return nil, err
	}

	codeResponse := make(chan string)
	closer := make(chan struct{})

	m := http.NewServeMux()
	m.HandleFunc("/confirm", func(w http.ResponseWriter, r *http.Request) {
		if x, ok := r.URL.Query()["code"]; ok {
			codeResponse <- x[0]
			close(codeResponse)
		} else {
			log.Println("Can't find code")
		}
	})

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println("Error in server start", err)
			} else {
				log.Println("Server shut down")
			}
		}()
		log.Printf("Starting web server at %s\n", u.Host)
		s := http.Server{
			Addr:    u.Host,
			Handler: m,
		}
		go func() {
			<-closer
			s.Close()
		}()
		if err := s.ListenAndServe(); err != nil {
			log.Println("Can't start the auth server callback", err)
		}
	}()

	code := <-codeResponse

	close(closer)

	form := url.Values{}
	form.Add("client_id", a.clientId)
	form.Add("scope", scopes)
	form.Add("code", code)
	form.Add("redirect_uri", a.callback)
	form.Add("grant_type", "authorization_code")
	form.Add("client_secret", a.clientSecret)
	resp, err := a.c.Post("https://login.microsoftonline.com/common/oauth2/v2.0/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		content, _ := ioutil.ReadAll(resp.Body)
		os.Stderr.Write(content)
		return nil, fmt.Errorf("Can't get tokens: %d : %s", resp.StatusCode, resp.Status)
	}

	var s Tokens

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (a *Auth) RefreshTokens(refreshToken string) (*Tokens, error) {
	form := url.Values{}
	form.Add("client_id", a.clientId)
	form.Add("scope", scopes)
	form.Add("refresh_token", refreshToken)
	form.Add("grant_type", "refresh_token")
	form.Add("client_secret", a.clientSecret)
	resp, err := a.c.Post("https://login.microsoftonline.com/common/oauth2/v2.0/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		content, _ := ioutil.ReadAll(resp.Body)
		os.Stderr.Write(content)
		return nil, fmt.Errorf("Can't get tokens: %d : %s", resp.StatusCode, resp.Status)
	}

	var s Tokens

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (a *Auth) GetLoginUrl() string {
	return fmt.Sprintf("https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=%s&response_type=code&redirect_uri=%s&response_mode=query&scope=%s&state=random", a.clientId, a.callback, scopes)
}
