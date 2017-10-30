package main

import (
	"fmt"
	"time"
	"strings"
	"encoding/json"

	"github.com/parnurzeal/gorequest"
	"gopkg.in/alecthomas/kingpin.v2"
	log "github.com/Sirupsen/logrus"
)

const (
	ver string = "0.10"
)

var (
	 timeout = kingpin.Flag("timeout", "timeout for HTTP requests in seconds").Default("15").Short('t').Int()
	 marathonURL = kingpin.Flag("url", "marathon URL").Required().Short('u').String()
	 basicAuthCreds = kingpin.Flag("basic-auth", "HTTP basic auth credentials").Short('a').String()
)

type Apps struct {
	Apps []App `json:"apps"`
}

type App struct {
	ID string `json:"id"`
	Instances int `json:"instances"`
	TasksStaged int `json:"tasksStaged"`
	TasksRunning int `json:"tasksRunning"`
	TasksHealthy int `json:"tasksHealthy"`
	TasksUnhealthy int `json:"tasksUnhealthy"`
}

type Msg struct {
	response string
	err error
}

func httpGet(url, creds string, ch chan Msg) {
	var msg Msg

	request := gorequest.New()
	if creds != "" {
		c := strings.Split(creds, ":")
		username := c[0]
		password := c[1]
		if username == "" || password == "" {
			msg.err = fmt.Errorf("Cannot parse basic auth credentials")
			ch <- msg
			return
		}
		request = gorequest.New().SetBasicAuth(username, password)
	}
	resp, body, errs := request.Get(url).End()

	if errs != nil {
		var errsStr []string
		for _, e := range errs {
			errsStr = append(errsStr, fmt.Sprintf("%s", e))
		}
		msg.err = fmt.Errorf("%s", strings.Join(errsStr, ", "))
		ch <- msg
		return
	}
	if resp.StatusCode == 200 {
		msg.response = body
	} else {
		msg.err = fmt.Errorf("HTTP response code: %s", resp.Status)
	}
	ch <- msg
}

func normalizeAppName(input string) string {
	output := strings.Replace(input, "/", "_", -1)
	if string(output[0]) == "_" {
		output = output[1:len(output)]
	}
	return output
}

func genLineProto(data []App) string {
	var output []string
	for _, v := range data {
		line := fmt.Sprintf(
			"marathon_apps,app_name=%s instances=%di,tasks_staged=%di,tasks_running=%di,tasks_healthy=%di,tasks_unhealthy=%di",
			normalizeAppName(v.ID),
			v.Instances,
			v.TasksStaged,
			v.TasksRunning,
			v.TasksHealthy,
			v.TasksUnhealthy,
		)
		output = append(output, line)
	}
	return strings.Join(output, "\n")
}

func parseJSONData(data string) []App {
	var a Apps
	err := json.Unmarshal([]byte(data), &a)
	if err != nil {
		log.Debug("Cannot parse JSON")
	}

	return a.Apps
}

func gatherMetrics(marathonURL, basicAuthCreds string, timeout int) string {
	ch := make(chan Msg)

	url := marathonURL + "/v2/apps"
	go httpGet(url, basicAuthCreds, ch)

	var results string
	var msg Msg
	select {
	case msg = <-ch:
		if msg.err == nil {
			results = msg.response
		} else {
			log.Error(msg.err)
		}
	case <-time.After(time.Second * time.Duration(timeout)):
		log.Error("Connection timeout")
	}
	return results
}

func main() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	kingpin.Version(ver)
	kingpin.Parse()

	fmt.Println(genLineProto(parseJSONData(gatherMetrics(*marathonURL, *basicAuthCreds, *timeout))))
}
