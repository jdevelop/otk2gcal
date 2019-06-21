package messaging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Sender interface {
	SendMessage(to string, msg string) error
}

type SlackSender struct {
	url    string
	client *http.Client
}

func NewSlackSender(url string, client *http.Client) *SlackSender {
	return &SlackSender{
		url:    url,
		client: client,
	}
}

type slackMsg struct {
	Text string `json:"text"`
}

type logger struct{}

var loggerVal logger

func (_ *logger) SendMessage(to string, msg string) error {
	log.Println(msg)
	return nil
}

func NewLogger() *logger {
	return &loggerVal
}

func (ss *SlackSender) SendMessage(to string, msg string) error {
	m := slackMsg{
		Text: msg,
	}
	data, err := json.Marshal(&m)
	if err != nil {
		return err
	}

	resp, err := ss.client.Post(ss.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Can't send message: %d : %s", resp.StatusCode, resp.Status)
	}
	return nil
}

var _ Sender = &SlackSender{}
var _ Sender = &loggerVal
