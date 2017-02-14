// https://www.opsdash.com/blog/slack-bot-in-golang.html

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var token string
var chrisify string

var filenames = []string{
	"have-a-good-day-have-it.jpg",
	"pro-smite-player.jpg",
	"esports-legend.jpg",
	"awkward-dancer.jpg",
}

func main() {
	rand.Seed(time.Now().UnixNano())
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: slackbot slack-bot-token /path/to/chrisify\n")
		os.Exit(1)
	}

	token = os.Args[1]
	chrisify = os.Args[2]

	// start a websocket-based Real Time API session
	ws, id := slackConnect(token)
	fmt.Println("slackbot ready, ^C exits")

	for {
		// read each incoming message
		m, err := getMessage(ws)
		if err != nil {
			log.Fatal(err)
		}

		// see if we're mentioned
		if m.Type == "message" && m.SubType == "file_share" && strings.Contains(m.Text, "<@"+id+">") {
			go func(m Message) {
				var channel string
				json.Unmarshal(m.Channel, &channel)
				file := SaveFile(GetFile(m.File))
				chrisd := Chrisify(file)
				log.Printf("Uploading to %s", channel)
				Upload(chrisd, channel)
				defer os.Remove(file)
			}(m)
		}
	}
}

func SaveFile(b []byte) string {
	file, err := ioutil.TempFile("", "slack_image")
	if err != nil {
		log.Fatalf("error saving file: %s", err)
	}
	if _, err = file.Write(b); err != nil {
		log.Fatalf("error writing file: %s", err)
	}
	if err = file.Close(); err != nil {
		log.Fatalf("error closing file: %s", err)
	}
	return file.Name()
}

func GetFile(file File) []byte {
	client := &http.Client{
		Timeout: time.Second * 20,
	}
	request, err := http.NewRequest(http.MethodGet, file.URLPrivateDownload, nil)
	if err != nil {
		log.Fatalf("error creating request: %s", err)
	}
	request.Header.Add("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("error downloading file\n%v\n%v", file, err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("error downloading file\n%v\n%v", file, err)
	}
	return body
}

func Chrisify(file string) []byte {
	out, err := exec.Command(chrisify, file).Output()
	if err != nil {
		log.Fatalf("couldn't chrisify: %s", err)
	}
	return out
}

func Upload(file []byte, channel string) {

	filename := filenames[rand.Intn(len(filenames))]

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		log.Fatalf("error creating form file: %s", err)
	}

	if _, err := io.Copy(fw, bytes.NewReader(file)); err != nil {
		log.Fatalf("error copying file into form buffer: %s", err)
	}

	w.WriteField("title", filename)
	w.WriteField("channels", channel)
	w.WriteField("token", token)

	w.Close()

	client := &http.Client{
		Timeout: time.Second * 20,
	}
	request, err := http.NewRequest(http.MethodPost, "https://slack.com/api/files.upload", &b)
	if err != nil {
		log.Fatalf("error creating request: %s", err)
	}

	request.Header.Set("Content-Type", w.FormDataContentType())

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("error doing request: %s", err)
	}

	if response.StatusCode != http.StatusOK {
		log.Fatalf("slack error: %s", response.Status)
	}
	return
}
