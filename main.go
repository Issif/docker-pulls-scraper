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

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type Stats struct {
	PullCount int `json:"pull_count"`
}

const (
	URLBase = "https://hub.docker.com/v2/repositories/"
)

func main() {
	list := flag.String("l", "", "File with the list of images to track")
	dataFolder := flag.String("d", "./data", "Destination folder for .csv")
	renderFolder := flag.String("r", "./render", "Destination folder for the .html")
	flag.Parse()

	if *list == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := os.Mkdir(*dataFolder, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
		log.Printf("Folder '%v' already exists\n", *dataFolder)
	}
	if err := os.Mkdir(*renderFolder, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
		log.Printf("Folder '%v' already exists\n", *renderFolder)
	}

	listFile, err := os.Open(*list)
	if err != nil {
		log.Fatal(err)
	}
	listFileScanner := bufio.NewScanner(listFile)
	listFileScanner.Split(bufio.ScanLines)

	for listFileScanner.Scan() {
		image := strings.ReplaceAll(listFileScanner.Text(), " ", "")
		// count := getPullCount(image)
		// writeCSV(image, count, *dataFolder)
		renderChart(image, *dataFolder, *renderFolder)
	}

}

func writeCSV(image string, count int, dataFolder string) {
	filename := fmt.Sprintf("%v/%v.csv", dataFolder, strings.ReplaceAll(image, "/", "_"))
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

func renderChart(image string, dataFolder, renderFolder string) {
	baseName := strings.ReplaceAll(image, "/", "_")
	log.Printf("Writing of the .html for the image '%v' in '%v'\n", image, fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	readFile, err := os.Open(fmt.Sprintf("%v/%v.csv", dataFolder, baseName))
	if err != nil {
		log.Fatal(err)
	}
	defer readFile.Close()

	xAxis := make([]string, 0)
	yAxis := make([]opts.LineData, 0)

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Scan()
	for fileScanner.Scan() {
		s := fileScanner.Text()
		xAxis = append(xAxis, strings.Split(s, ",")[0])
		yAxis = append(yAxis, opts.LineData{Value: strings.Split(s, ",")[1]})
	}

	bar := charts.NewLine()
	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title: image,
	}))

	bar.SetXAxis(xAxis).
		AddSeries("# pulls", yAxis)
	f, _ := os.Create(fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	bar.Render(f)
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
