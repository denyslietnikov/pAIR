package cmd

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const (
	currentSongEndpoint = "/command/?cmd=get_currentsong"
	pollingInterval     = 5 * time.Second
)

var currentSongURL = ""

type SongResponse struct {
	Title string `json:"title"`
}

func getCurrentSong() (SongResponse, error) {
	currentSongURL = cfg.MoodeBaseURL + currentSongEndpoint
	resp, err := http.Get(currentSongURL)
	if err != nil {
		return SongResponse{}, err
	}
	defer resp.Body.Close()

	var songResponse SongResponse
	err = json.NewDecoder(resp.Body).Decode(&songResponse)
	if err != nil {
		return SongResponse{}, err
	}

	return songResponse, nil
}

func runSongService(fileContentChan chan<- string) {
	for {
		songResponse, err := getCurrentSong()
		if err != nil {
			log.Printf("Error getting current song: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		fileContentChan <- songResponse.Title
		log.Printf("Title: %s", songResponse.Title)
		time.Sleep(pollingInterval)
	}
}
