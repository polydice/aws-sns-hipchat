package main

import (
	"github.com/andybons/hipchat"
	"github.com/go-martini/martini"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"bytes"
	"strconv"
)

type Notification struct {
  Message string
  MessageId string
  Signature string
  SignatureVersion string
  SigningCertURL string
  SubscribeURL string
  Subject string
  Timestamp string
  TopicArn string
  Type string
  UnsubscribeURL string
}

type HipChatSender struct{
  AuthToken string
}

func (h HipChatSender)SendMessage(room_id, message string) error {
  c := hipchat.Client{AuthToken: h.AuthToken}
  req := hipchat.MessageRequest{
    RoomId:        room_id,
    From:          "Amazon SNS",
    Message:       message,
    Color:         hipchat.ColorYellow,
    MessageFormat: hipchat.FormatText,
    Notify:        true,
  }

  return c.PostMessage(req)
}

func TriggerJob(job_name string, n Notification) {

	apiUrl := "http://jenkins.ifeelgoods.com/buildByToken/build?job=" + job_name

	data := url.Values{}
	data.Add("token", jenkins_token)

	data.Add("Message", n.Message)
	data.Add("Subject", n.Subject)

	u, _ := url.ParseRequestURI(apiUrl)
	urlStr := fmt.Sprintf("%v", u)

	fmt.Printf("%v\n", urlStr)

	client := &http.Client{}
	r, _ := http.NewRequest("POST", urlStr, bytes.NewBufferString(data.Encode())) // <-- URL-encoded payload
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, _ := client.Do(r)
	fmt.Println(resp.Status)
}

func SnsJenkins(args martini.Params, w http.ResponseWriter, r *http.Request) {
	job_name := args["job_name"]

	var n Notification
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&n)

	if (err != nil) {
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received notification job_name:%v notification:%+v\n", job_name, n)

	if s := n.SubscribeURL; len(s) != 0 {
		fmt.Printf("SubscribeURL detected: %v\n", s)

		if _, err := http.Get(s); err != nil {
			fmt.Printf("Subscribe error: %v\n", err)
		}
	}

	TriggerJob(job_name, n)
}

func ServeHTTP(args martini.Params, w http.ResponseWriter, r *http.Request, h HipChatSender) {
  room_id := args["room_id"]

  var n Notification
  dec := json.NewDecoder(r.Body)
  err := dec.Decode(&n)

  if (err != nil) {
    http.Error(w, "Invalid JSON.", http.StatusBadRequest)
    return
  }

  fmt.Printf("Received notification room_id:%v notification:%+v\n", room_id, n)

  if s := n.SubscribeURL; len(s) != 0 {
    fmt.Printf("SubscribeURL detected: %v\n", s)

    if _, err := http.Get(s); err != nil {
      fmt.Printf("Subscribe error: %v\n", err)
    }
  }

  if len(n.Message) != 0 && len(n.Subject) != 0 {
    err := h.SendMessage(room_id, fmt.Sprintf("%v: %v", n.Subject, n.Message))
    if err != nil {
      fmt.Printf("HipChat error: %v\n", err)
      http.Error(w, err.Error(), http.StatusInternalServerError)
    }
  }
}

var jenkins_token string

func main() {
	fmt.Println("Starting aws-sns proxy server.")
	m := martini.Classic()
	h := HipChatSender{AuthToken: os.Getenv("HIPCHAT_AUTH_TOKEN")}
	jenkins_token = os.Getenv("JENKINS_TOKEN")

	m.Map(h)

	m.Post("/sns/hipchat/:room_id", ServeHTTP)
	m.Post("/sns/jenkins/:job_name", SnsJenkins)
	m.Run()
}
