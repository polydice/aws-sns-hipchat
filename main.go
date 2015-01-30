package main

import (
  "github.com/andybons/hipchat"
  "encoding/json"
  "fmt"
  "net/http"
  "os"
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
  c.BaseURL = "https://api.hipchat.com/v1"
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

func (h HipChatSender) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  room_id := r.URL.Path[1:]

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

func main() {
  fmt.Println("Starting aws-sns-hipchat server.")

  h := HipChatSender{AuthToken: os.Getenv("HIPCHAT_AUTH_TOKEN")}
  http.ListenAndServe(":"+os.Getenv("PORT"), h)
}
