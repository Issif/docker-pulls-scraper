package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Stats struct {
	PullCount int `json:"pull_count"`
}

const (
	URLBase = "https://hub.docker.com/v2/repositories/"
)

func main() {
	list := flag.String("l", "", "File with the list of images to track")
	folder := flag.String("f", "./outputs", "Destination folder for .csv")
	flag.Parse()

	if *list == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := os.Mkdir("outputs", os.ModePerm); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
		log.Printf("Folder '%v' already exists\n", *folder)
	}

	listFile, err := os.Open(*list)
	if err != nil {
		log.Fatal(err)
	}
	listFileScanner := bufio.NewScanner(listFile)
	listFileScanner.Split(bufio.ScanLines)

	for listFileScanner.Scan() {
		image := strings.ReplaceAll(listFileScanner.Text(), " ", "")
		count := getPullCount(image)
		writeCSV(image, count, *folder)
	}

}

func writeCSV(image string, count int, folder string) {
	filename := fmt.Sprintf("%v/%v.csv", folder, strings.ReplaceAll(image, "/", "_"))
	log.Printf("Writing of the .csv for the image '%v' in '%v'\n", image, filename)

	_, errorExist := os.Stat(filename)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer f.Close()

	if os.IsNotExist(errorExist) {
		if _, err := f.WriteString("Date,Count\n"); err != nil {
			log.Fatal(err)
			return
		}
	}

	if _, err := f.WriteString(fmt.Sprintf("%v,%v\n", time.Now().Format("2006/01/02"), count)); err != nil {
		log.Fatal(err)
		return
	}
}

func getPullCount(image string) int {
	log.Printf("Scrape pull count for the image '%v'\n", image)
	res, err := http.Get(URLBase + image)
	if err != nil {
		log.Fatal(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var s Stats
	json.Unmarshal(body, &s)
	return s.PullCount
}
