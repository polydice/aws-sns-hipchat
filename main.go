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
	"io/ioutil"
	"github.com/google/go-querystring/query"
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

type AutoScalingNotification struct {
	Service string `url:"Service"`
	Time string `url:"Time"`
	RequestId string `url:"RequestId"`
	Event string `url:"Event"`
	AccountId string `url:"AccountId"`
	AutoScalingGroupName string `url:"AutoScalingGroupName"`
	AutoScalingGroupARN string `url:"AutoScalingGroupARN"`
	ActivityId string `url:"ActivityId"`
	Description string `url:"Description"`
	Cause string `url:"Cause"`
	StartTime string `url:"StartTime"`
	EndTime string `url:"EndTime"`
	StatusCode string `url:"StatusCode"`
	StatusMessage string `url:"StatusMessage"`
	Progress string `url:"Progress"`
	EC2InstanceId string `url:"EC2InstanceId"`
	Details string `url:"Details"`
	UnsubscribeURL string `url:"UnsubscribeURL"`
	SubscribeURL string `url:"SubscribeURL"`
	Token string `url:"token"`
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

func TriggerJob(job_name string, n AutoScalingNotification) {

	apiUrl := "http://jenkins.ifeelgoods.com/buildByToken/buildWithParameters?job=" + job_name

	n.Token = jenkins_token

	v, _ := query.Values(n)
	data := v.Encode()

	u, _ := url.ParseRequestURI(apiUrl)
	urlStr := fmt.Sprintf("%v", u)

	fmt.Printf("%v\n", urlStr)

	client := &http.Client{}
	r, _ := http.NewRequest("POST", urlStr, bytes.NewBufferString(data)) // <-- URL-encoded payload
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data)))

	resp, _ := client.Do(r)
	fmt.Println(resp.Status)

	defer resp.Body.Close()

	content, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(content))
}

func SnsJenkins(args martini.Params, w http.ResponseWriter, r *http.Request) {
	job_name := args["job_name"]

	var n AutoScalingNotification
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

	if n.Event == "autoscaling:EC2_INSTANCE_LAUNCH" {
		TriggerJob(job_name, n)
	}else{
		fmt.Printf("Trigger error, this is not the right event to launch an ec2 instance: %v\n", n.Event )
	}
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
