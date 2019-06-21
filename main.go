package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"msclnd/auth"
	"msclnd/calendar"
	"msclnd/messaging"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

var interval = flag.String("interval", "1h", "Event interval")

//	callback     = flag.String("oauthcb", "http://localhost:8083/confirm", "OAuth callback url")
type config struct {
	Callback     string      `json:"callback"`
	ClientId     string      `json:"client_id"`
	ClientSecret string      `json:"client_secret"`
	SlackUrl     string      `json:"slack_url"`
	Tokens       auth.Tokens `json:"tokens"`
}

func main() {

	flag.Parse()

	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	confPath := filepath.Join(u.HomeDir, ".msclndrc")

	var c config

	confFile, err := os.Open(confPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := json.NewDecoder(confFile).Decode(&c); err != nil {
		log.Fatal(err)
	}

	confFile.Close()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	a, err := auth.New(c.ClientId, c.ClientSecret, c.Callback, client)
	if err != nil {
		log.Fatal(err)
	}

	if c.Tokens.RefreshToken != "" {
		if t, err := a.RefreshTokens(c.Tokens.RefreshToken); err != nil {
			log.Fatalf("Can't refresh tokens from %+v\n", err)
		} else {
			c.Tokens = *t
		}
	} else {
		fmt.Printf("Please login at: %s\n", a.GetLoginUrl())
		t, err := a.GetTokens()
		if err != nil {
			log.Fatal(err)
		}
		c.Tokens = *t
	}

	confData, err := json.Marshal(&c)
	if err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(confPath, confData, 0600); err != nil {
		log.Fatal(err)
	}

	cal := calendar.New(client)
	current := time.Now().UTC()

	intervalTime := 1 * time.Hour

	if *interval != "" {
		d, err := time.ParseDuration(*interval)
		if err != nil {
			log.Fatal(err)
		}
		intervalTime = d
	}

	events, err := cal.GetMyEvents(&c.Tokens, current, current.Add(intervalTime))
	if err != nil {
		log.Fatal(err)
	}

	if len(events) == 0 {
		return
	}

	log.Printf("Found %d events\n", len(events))

	var msg strings.Builder

	for _, v := range events {
		msg.WriteString(fmt.Sprintf("%s at %s / %s\n", v.Subject, v.Start.Format(time.RFC3339), v.End.Format(time.RFC3339)))
	}

	var sender messaging.Sender

	if c.SlackUrl != "" {
		sender = messaging.NewSlackSender(c.SlackUrl, client)
	} else {
		sender = messaging.NewLogger()
	}

	sender.SendMessage("", msg.String())

}
