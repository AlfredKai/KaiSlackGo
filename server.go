package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type ConnectionResponse struct {
	Ok  bool   `json:"ok"`
	Url string `json:"url"`
}

type messageResponse struct {
	EnvelopeId  string   `json:"envelope_id"`
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	err := godotenv.Load(".env")

	if err != nil {
	  log.Fatalf("Error loading .env file")
	}

	httpClient := http.Client{Timeout: time.Duration(1) * time.Second}
	req, err := http.NewRequest("POST", "https://slack.com/api/apps.connections.open", nil)
	if err != nil {
		log.Printf("error %s", err)
		return
	}
	appToken := os.Getenv("APP_TOKEN")
	req.Header.Add("Content-type", `application/x-www-form-urlencoded`)
	req.Header.Add("Authorization", `Bearer ` + appToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error %s", err)
		return
	}

	defer resp.Body.Close()

	var cResp ConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&cResp); err != nil {
		log.Fatal("Errorn %s", err)
	}

	log.Printf("Ok %t Url %s", cResp.Ok, cResp.Url)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	c, _, err := websocket.DefaultDialer.Dial(cResp.Url, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)

			var mResp messageResponse
			if err := json.Unmarshal([]byte(message), &mResp); err != nil {
				log.Fatal("json %s", err)
			}

			if mResp.EnvelopeId != "" {
				log.Printf("envelope_id: %s", mResp.EnvelopeId)

				ack, err := json.Marshal(map[string]string{
					"envelope_id":  mResp.EnvelopeId,
				})
				if err != nil {
					log.Println("ack:", err)
					return
				}
				log.Printf("envelope_id: %s", ack)
	
				err = c.WriteMessage(websocket.TextMessage, ack)
				if err != nil {
					log.Println("write:", err)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			}
			return
		}
	}
}
