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
	"encoding/pem"
	"crypto/x509"
	"encoding/base64"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"log"
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
	Progress int `url:"Progress"`
	EC2InstanceId string `url:"EC2InstanceId"`
	//Details string `url:"Details"`
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


func CheckSignature(certUrl string, signature_64 string, raw_string string) bool {

	// This will use https and so verify that the certificate comes from Amazon
	resp, err := http.Get(certUrl)
	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()

	pemPublicKey, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(pemPublicKey))

	// Parse public key into rsa.PublicKey
	PEMBlock, _ := pem.Decode([]byte(pemPublicKey))
	if PEMBlock == nil {
		log.Fatal("Could not parse Public Key PEM")
	}
	if PEMBlock.Type != "CERTIFICATE" {
		fmt.Printf("%s\n", PEMBlock.Type)
		log.Fatal("Found wrong key type")
	}

	certificate, err := x509.ParseCertificate(PEMBlock.Bytes)
	if err != nil {
		log.Fatal(err)
	}

	signature, err := base64.StdEncoding.DecodeString(signature_64)

	if err != nil {
		fmt.Println(err)
	}

	h := sha1.New()
	h.Write([]byte(raw_string))

	// Verify
	pub := certificate.PublicKey.(*rsa.PublicKey)

	fmt.Println(certificate.Signature)

	err = rsa.VerifyPKCS1v15(pub, crypto.SHA1, h.Sum(nil), []byte(signature))
	if err != nil {
		log.Fatal(err)
	}


	return true
}

func VerifyNotification(n Notification) {
	signString := fmt.Sprintf(`Message
%v
MessageId
%v`, n.Message, n.MessageId)

	if n.Subject != "" {
		signString = signString + fmt.Sprintf(`
Subject
%v`, n.Subject)
	}

	signString = signString + fmt.Sprintf(`
Timestamp
%v
TopicArn
%v
Type
%v
`, n.Timestamp, n.TopicArn, n.Type)

	certUrl := "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem"
	signature := "br3KcAFiJ6+o4J3JOFtVo/84osp/i3UOh8SRwCoa95vhNyrxtCD/WDi/gxGUv0Kuh4Y5VQJcvzP/KOSCzYRS3jY2RNgV1unIso+FrE3PDFO9SLjF5mcUwReV7jwGSGEovC+lveew6jqas4/hboJGheZCiFCjFSNnW4FPx2iOXLcNMTp+6uHZGF2rwoB+FO2qyNuKQmRM4rAeKlOvC/yaoBIwVbjYpD4EPjnibLfZyV8CGua7uHLnano4fdZKsJ0L4oIwXWTI+e19WlmtipA+gkl152/oFX+wwqUjahTnnyOD3XDW6XxK1fiOJEquAhaUJVBhtZNyQI3SyC955Irz8g=="


	fmt.Sprintln(CheckSignature(certUrl, signature, signString))
}

func TriggerJob(job_name string, n AutoScalingNotification) {

	apiUrl := jenkins_url + "/buildByToken/buildWithParameters?job=" + job_name

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

	var notif Notification
	var autoScalNotif AutoScalingNotification

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&notif)

	if (err != nil) {
		content, _ := ioutil.ReadAll(r.Body)
		fmt.Printf("%s\n", string(content))
		fmt.Println(err)

		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	message_byte := []byte(notif.Message)
	err = json.Unmarshal(message_byte, &autoScalNotif)

	if (err != nil) {
		fmt.Printf("%s\n", notif.Message)
		fmt.Println(err)
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received notification job_name:%v notification:%+v\n", job_name, autoScalNotif)

	if s := notif.SubscribeURL; len(s) != 0 {
		fmt.Printf("SubscribeURL detected: %v\n", s)

		if _, err := http.Get(s); err != nil {
			fmt.Printf("Subscribe error: %v\n", err)
		}
	}

	if autoScalNotif.Event == "autoscaling:EC2_INSTANCE_LAUNCH" {
		TriggerJob(job_name, autoScalNotif)
	}else{
		fmt.Printf("Trigger error, this is not the right event to launch an ec2 instance: %v\n", autoScalNotif.Event )
	}
}

func ServeHTTP(args martini.Params, w http.ResponseWriter, r *http.Request, h HipChatSender) {
  room_id := args["room_id"]

  var n Notification
  dec := json.NewDecoder(r.Body)
  err := dec.Decode(&n)

  if (err != nil) {
	fmt.Println(err)
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
var jenkins_url string

func main() {
	fmt.Println("Starting aws-sns proxy server.")
	m := martini.Classic()
	h := HipChatSender{AuthToken: os.Getenv("HIPCHAT_AUTH_TOKEN")}
	jenkins_token = os.Getenv("JENKINS_TOKEN")
	jenkins_url = os.Getenv("JENKINS_URL")
	m.Map(h)

	m.Post("/sns/hipchat/:room_id", ServeHTTP)
	m.Post("/sns/jenkins/:job_name", SnsJenkins)
	m.Run()
}
