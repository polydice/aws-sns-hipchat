aws-sns-hipchat
===============

A simple AWS SNS HTTP(s) endpoint to send notification to HipChat chatroom. 

## Features

* Designed to run on Heroku
* Automatically confirm subscription
* Send notifications to multiple chatrooms
* Heroku-compatible logs for every notification and subscribe confirmation

The HTTP(s) API is `http://aws-sns-hipchat.herokuapp.com/ROOM_ID`, which could be used to register as endpoints on SNS.

## Heroku Deployment

[kr/heroku-buildpack-go](https://github.com/kr/heroku-buildpack-go) is used for running this project on Heroku, to create Heorku app with the build pack: 

```bash
heroku create -b https://github.com/kr/heroku-buildpack-go.git
```

If you're not familiar with using Go on Heroku, check [Getting Started with Go on Heroku](http://mmcgrana.github.io/2012/09/getting-started-with-go-on-heroku.html) for more details.

## Configuration

To make this app up and work properly, you need to set an environment variable `HIPCHAT_AUTH_TOKEN` as your API auth token of HipChat:

```bash
heroku config:add HIPCHAT_AUTH_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Contact

Richard Lee

- http://github.com/dlackty
- http://twitter.com/dlackty
- dlackty@gmail.com

## License

`aws-sns-hipchat` is available under the MIT license. See the `LICENSE` file for more info.
