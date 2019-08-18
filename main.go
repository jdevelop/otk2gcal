package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jdevelop/otk2gcal/auth"
	"github.com/jdevelop/otk2gcal/calendar"
	"github.com/jdevelop/otk2gcal/messaging"
	"github.com/jdevelop/otk2gcal/storage"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	interval   = flag.String("interval", "1h", "Event interval")
	dbPath     = flag.String("db", "", "database path")
	calendarId = flag.String("calendarId", "", "Google calendar id")
)

type config struct {
	OutlookConf   auth.OutlookConf   `json:"outlook_conf"`
	GoogleCalConf auth.GoogleCalConf `json:"google_cal_conf"`
	SlackUrl      string             `json:"slack_url"`
}

func initDataDir(u *user.User) (string, error) {
	dir := filepath.Join(u.HomeDir, ".msclnd")
	fi, err := os.Stat(dir)
	switch {
	case err != nil:
		switch {
		case os.IsNotExist(err):
			if err := os.MkdirAll(dir, 0700); err != nil {
				return "", errors.Wrapf(err, "can't create foler %s", dir)
			}
			return dir, nil
		}
	default:
		if !fi.IsDir() {
			return "", fmt.Errorf("not a folder %s", dir)
		}
	}
	return dir, nil
}

func initOutlookCalendar() {
}

func main() {

	flag.Parse()

	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	confPath := filepath.Join(u.HomeDir, ".msclndrc")

	dir, err := initDataDir(u)
	if err != nil {
		log.Fatalf("can't initiaize folder %+v", err)
	}

	if *dbPath == "" {
		*dbPath = filepath.Join(dir, "msclndrc.db")
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

	outlookTokens, err := auth.NewOutlookTokens(c.OutlookConf, client, filepath.Join(dir, "outlook.json"))
	if err != nil {
		log.Fatal(err)
	}

	googleCal, err := calendar.NewGoogleCal(c.GoogleCalConf, filepath.Join(dir, "google.json"), *calendarId)
	if err != nil {
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

	events, err := cal.GetMyEvents(outlookTokens, startTime, endTime)
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
	toSync := make([]calendar.Event, 0)

	for _, v := range events {
		exists, err := idsStorage.IsExist(v.Id)
		if err != nil {
			sender.SendMessage("", fmt.Sprintf("Can't look up ID %s: %+v", v.Id, err))
			os.Exit(1)
		}

		if !exists {
			msg.WriteString(fmt.Sprintf("*%s* at *%s* [ %s ]\n", v.Subject, v.Start.Format(time.RFC822), v.End.Sub(*v.Start).String()))
			toSend = append(toSend, v.Id)
			toSync = append(toSync, v)
		}
	}

	if len(toSend) > 0 {
		sender.SendMessage("", msg.String())
		if err := idsStorage.AddEvents(toSend...); err != nil {
			sender.SendMessage("", fmt.Sprintf("Can't add ids to the storage %+v", err))
		}
		if err := googleCal.AddEvents(toSync); err != nil {
			log.Printf("Can't sync calendar: %+v", err)
		}
	}

}
