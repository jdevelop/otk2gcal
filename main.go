package main

import (
	"flag"
	"fmt"
	"log"
	"msclnd/auth"
	"msclnd/calendar"
	"msclnd/storage"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

var (
	callback     = flag.String("oauthcb", "http://localhost:8083/confirm", "OAuth callback url")
	clientId     = flag.String("client_id", "", "Office client id")
	clientSecret = flag.String("client_secret", "", "Office client secret")
	refreshToken = flag.String("refresh_token", "", "Refresh token")
	tokenPath    = flag.String("tokenPath", "", "Token file path")
)

func main() {
	flag.Parse()

	if *tokenPath == "" {
		u, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		*tokenPath = filepath.Join(u.HomeDir, ".msclndrc")
	}

	log.Printf("Using token path: %s\n", *tokenPath)

	_, err := os.Stat(*tokenPath)
	configNotFound := err != nil && os.IsNotExist(err)

	if *clientId == "" || *clientSecret == "" {
		log.Println("client id and secret must be specified")
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	a, err := auth.New(*clientId, *clientSecret, *callback, client)
	if err != nil {
		log.Fatal(err)
	}

	var (
		tokens *auth.Tokens
		fst    = storage.NewFileStorage(*tokenPath)
	)

	if !configNotFound {
		if t, err := fst.LoadTokens(); err != nil {
			log.Fatalf("Can't load/read tokens %s: %+v\n", *tokenPath, err)
		} else {
			tokens = t
		}
		if t, err := a.RefreshTokens(tokens.RefreshToken); err != nil {
			log.Fatalf("Can't refresh tokens from %s: %+v\n", *tokenPath, err)
		} else {
			if err := fst.SaveTokens(t); err != nil {
				log.Fatalf("Can't save tokens into %s: %+v\n", *tokenPath, err)
			}
			tokens = t
		}
	} else {
		if *refreshToken == "" {
			fmt.Printf("Please login at: %s\n", a.GetLoginUrl())
			t, err := a.GetTokens()
			if err != nil {
				log.Fatal(err)
			}
			tokens = t
		} else {
			t, err := a.RefreshTokens(*refreshToken)
			if err != nil {
				log.Fatal(err)
			}
			tokens = t
		}
		fst.SaveTokens(tokens)
		log.Printf("Saved tokens to %s\n", *tokenPath)
	}

	cal := calendar.New(client)
	current := time.Now().UTC()
	events, err := cal.GetMyEvents(tokens, current, current.AddDate(0, 0, 1))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Found %d events\n", len(events))

	for _, v := range events {
		log.Printf("Event: %s at %s/%s\n", v.Subject, v.Start.Format(time.RFC3339), v.End.Format(time.RFC3339))
	}

}
