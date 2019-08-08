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

	"github.com/pkg/errors"
)

const (
	scopes = "Calendars.ReadWrite User.Read offline_access openid"
)

type OutlookConf struct {
	Callback     string `json:"callback"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type OutlookTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func GetTokens(conf OutlookConf, client *http.Client) (*OutlookTokens, error) {
	u, err := url.Parse(conf.Callback)
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
	form.Add("client_id", conf.ClientId)
	form.Add("scope", scopes)
	form.Add("code", code)
	form.Add("redirect_uri", conf.Callback)
	form.Add("grant_type", "authorization_code")
	form.Add("client_secret", conf.ClientSecret)
	resp, err := client.Post("https://login.microsoftonline.com/common/oauth2/v2.0/token",
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

	var s OutlookTokens

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func RefreshTokens(refreshToken string, conf OutlookConf, client *http.Client) (*OutlookTokens, error) {
	form := url.Values{}
	form.Add("client_id", conf.ClientId)
	form.Add("scope", scopes)
	form.Add("refresh_token", refreshToken)
	form.Add("grant_type", "refresh_token")
	form.Add("client_secret", conf.ClientSecret)
	resp, err := client.Post("https://login.microsoftonline.com/common/oauth2/v2.0/token",
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

	var s OutlookTokens

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func GetLoginUrl(conf OutlookConf) string {
	return fmt.Sprintf("https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=%s&response_type=code&redirect_uri=%s&response_mode=query&scope=%s&state=random", conf.ClientId, conf.Callback, scopes)
}

func NewOutlookTokens(conf OutlookConf, client *http.Client, tokenPath string) (*OutlookTokens, error) {
	var tokens OutlookTokens

	outlookTokensData, err := ioutil.ReadFile(tokenPath)

	switch {
	case err != nil:
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "can't read tokens from %s", tokenPath)
		}
	default:
		if err := json.Unmarshal(outlookTokensData, &tokens); err != nil {
			return nil, errors.Wrapf(err, "can't unmarshal tokens from %s", tokenPath)
		}
	}

	if tokens.RefreshToken != "" {
		if t, err := RefreshTokens(tokens.RefreshToken, conf, client); err != nil {
			return nil, errors.Wrap(err, "can't refresh tokens")
		} else {
			tokens = *t
		}
	} else {
		fmt.Printf("Please login at: %s\n", GetLoginUrl(conf))
		t, err := GetTokens(conf, client)
		if err != nil {
			return nil, errors.Wrap(err, "can't authenticate")
		}
		tokens = *t
	}

	newOutlookTokens, err := json.Marshal(&tokens)
	if err != nil {
		return nil, errors.Wrapf(err, "can't marshal tokens")
	}

	if err := ioutil.WriteFile(tokenPath, newOutlookTokens, 0600); err != nil {
		return nil, errors.Wrapf(err, "can't write tokens into %s", tokenPath)
	}

	return &tokens, nil
}
