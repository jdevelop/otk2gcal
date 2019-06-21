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
	"msclnd/storage"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	interval = flag.String("interval", "1h", "Event interval")
	dbPath   = flag.String("db", "", "database path")
)

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
	if *dbPath == "" {
		*dbPath = filepath.Join(u.HomeDir, ".msclndrc.db")
	}

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

	startTime, endTime := current, current.Add(intervalTime)
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	events, err := cal.GetMyEvents(&c.Tokens, startTime, endTime)
	if err != nil {
		log.Fatal(err)
	}

	if len(events) == 0 {
		return
	}

	log.Printf("Found %d events\n", len(events))

	var sender messaging.Sender

	if c.SlackUrl != "" {
		sender = messaging.NewSlackSender(c.SlackUrl, client)
	} else {
		sender = messaging.NewLogger()
	}

	var (
		msg        strings.Builder
		idsStorage storage.EventsStorage
	)

	if s, err := storage.NewBoltEventStorage(*dbPath); err != nil {
		sender.SendMessage("", fmt.Sprintf("Can't open storage: %+v", err))
		idsStorage = storage.NewNoop()
	} else {
		idsStorage = s
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(*events[j].Start)
	})

	toSend := make([]string, 0, len(events))

	for _, v := range events {
		exists, err := idsStorage.IsExist(v.Id)
		if err != nil {
			sender.SendMessage("", fmt.Sprintf("Can't look up ID %s: %+v", v.Id, err))
			os.Exit(1)
		}

		if !exists {
			msg.WriteString(fmt.Sprintf("*%s* at *%s* [ %s ]\n", v.Subject, v.Start.Format(time.RFC822), v.End.Sub(*v.Start).String()))
			toSend = append(toSend, v.Id)
		}
	}

	if len(toSend) > 0 {
		sender.SendMessage("", msg.String())
		if err := idsStorage.AddEvents(toSend...); err != nil {
			sender.SendMessage("", fmt.Sprintf("Can't add ids to the storage %+v", err))
		}
	}

}
