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
	"strings"
	"regexp"
)

type Configuration struct {
	Colors map[string]string
}

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

func (h HipChatSender)SendMessage(room_id, message string, color string) error {
  c := hipchat.Client{AuthToken: h.AuthToken}
  req := hipchat.MessageRequest{
    RoomId:        room_id,
    From:          "Amazon SNS",
    Message:       message,
    Color:         color,
    MessageFormat: hipchat.FormatText,
    Notify:        true,
  }

  return c.PostMessage(req)
}


func CheckSignature(certUrl string, signature_64 string, raw_string string) bool {

	// This will use https and so verify that the certificate comes from Amazon
	urlParsed, err := url.Parse(certUrl)
	if err != nil {
		fmt.Println(err)
		return false
	}

	domain := strings.Split(urlParsed.Host, ":")[0]
	top_domains := strings.Split(domain, ".")
	nb_domain_part := len(top_domains)

	if (top_domains[nb_domain_part-1] == "com") && (top_domains[nb_domain_part-2] == "amazonaws") {
		fmt.Printf("Domain verified\n")
	} else {
		fmt.Printf("Domain invalid%s\n", top_domains)
		return false
	}

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
		return false
	}
	if PEMBlock.Type != "CERTIFICATE" {
		fmt.Printf("%s\n", PEMBlock.Type)
		log.Fatal("Found wrong key type")
		return false
	}

	certificate, err := x509.ParseCertificate(PEMBlock.Bytes)
	if err != nil {
		log.Fatal(err)
		return false
	}

	signature, err := base64.StdEncoding.DecodeString(signature_64)

	if err != nil {
		fmt.Println(err)
		return false
	}

	h := sha1.New()
	h.Write([]byte(raw_string))

	// Verify
	pub := certificate.PublicKey.(*rsa.PublicKey)

	err = rsa.VerifyPKCS1v15(pub, crypto.SHA1, h.Sum(nil), []byte(signature))
	if err != nil {
		log.Fatal(err)
		return false
	} else {
		fmt.Printf("Succeed to verify the signature")
	}

	return true
}

func VerifyNotification(n Notification) bool {
	signString := fmt.Sprintf(`Message
%v
MessageId
%v`, n.Message, n.MessageId)

	if n.Subject != "" {
		signString = signString+fmt.Sprintf(`
Subject
%v`, n.Subject)
	}

	signString = signString+fmt.Sprintf(`
Timestamp
%v
TopicArn
%v
Type
%v
`, n.Timestamp, n.TopicArn, n.Type)

	return CheckSignature(n.SigningCertURL, n.Signature, signString)
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

//	// Debug purpose
//	bodyIo, errBody := ioutil.ReadAll(r.Body);
//
//	if errBody != nil {
//		fmt.Printf("Error when reading body : %s", errBody)
//	}
//
//	fmt.Printf("Body of the request: %s", bodyIo)
//	// end debug

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
	} else {
		if VerifyNotification(notif) != true {
			fmt.Printf("%s\n", notif.Message)
			fmt.Printf("Failed to verify signature")
			http.Error(w, "Not Authorized", http.StatusBadRequest)
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
		color := ColorStyle(n.Subject)
    err := h.SendMessage(room_id, fmt.Sprintf("%v: %v", n.Subject, n.Message), color)
    if err != nil {
      fmt.Printf("HipChat error: %v\n", err)
      http.Error(w, err.Error(), http.StatusInternalServerError)
    }
  }
}

func ColorStyle(text string) string {
	style := hipchat.ColorGray
	for pattern, color := range config.Colors {
		r, _ := regexp.Compile(pattern)
		if r.MatchString(text) {
			style = color
			break
		}
	}
		return style
}

func LoadConfiguration() Configuration {
	file, err := os.Open("conf.json")
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(configuration.Colors)
	return configuration
}

var jenkins_token string
var jenkins_url string
var config Configuration

func main() {
	fmt.Println("Starting aws-sns proxy server.")
	m := martini.Classic()
	h := HipChatSender{AuthToken: os.Getenv("HIPCHAT_AUTH_TOKEN")}
	jenkins_token = os.Getenv("JENKINS_TOKEN")
	jenkins_url = os.Getenv("JENKINS_URL")
	config = LoadConfiguration()
	m.Map(h)

	m.Post("/sns/hipchat/:room_id", ServeHTTP)
	m.Post("/sns/jenkins/:job_name", SnsJenkins)
	m.Run()
}
