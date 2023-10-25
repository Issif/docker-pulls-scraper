package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type Image struct {
	Name       string
	Count      int64 `json:"pull_count"`
	CommaCount string
}

var images []Image

const (
	URLBase = "https://hub.docker.com/v2/repositories/"
)

func init() {
	images = make([]Image, 0)
}

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

	for listFileScanner.Scan() {
		i := strings.ReplaceAll(listFileScanner.Text(), " ", "")
		getPullCount(i)
	}

	for _, i := range images {
		writeCSV(i, *dataFolder)
		renderChart(i, *dataFolder, *renderFolder)
	}

	updateIndexHTML()
}

func writeCSV(image Image, dataFolder string) {
	filename := fmt.Sprintf("%v/%v.csv", dataFolder, strings.ReplaceAll(image.Name, "/", "_"))
	log.Printf("Writing of the .csv for the image '%v' in '%v'\n", image.Name, filename)

	_, errorExist := os.Stat(filename)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if os.IsNotExist(errorExist) {
		if _, err := f.WriteString("Date,Count\n"); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := f.WriteString(fmt.Sprintf("%v,%v\n", time.Now().Format("2006/01/02"), image.Count)); err != nil {
		log.Fatal(err)
	}
}

func renderChart(image Image, dataFolder, renderFolder string) {
	baseName := strings.ReplaceAll(image.Name, "/", "_")
	log.Printf("Writing of the .html for the image '%v' in '%v'\n", image.Name, fmt.Sprintf("%v/%v.html", renderFolder, baseName))
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

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(
			opts.Initialization{
				PageTitle: image.Name},
		),
		charts.WithTitleOpts(
			opts.Title{
				Title: image.Name,
			}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
	)

	line.SetXAxis(xAxis).AddSeries("# pulls", yAxis)
	f, _ := os.Create(fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	line.Render(f)
}

func updateIndexHTML() {
	log.Println("Writing of the index.html")

	templateStr := `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css">
	<title>Docker pull counts</title>
</head>
<body>
	<div class="row">
		<div class="col s3">
		<table class="striped responsive-table" style="margin: 20px">
			<thead>
				<tr>
					<th>Image</th>
					<th>Last count</th>
					<th>Chart</th>
				</tr>
			</thead>
			<tbody>
				{{- range . }}
				<tr>
					<td><a href="https://hub.docker.com/r/{{ .Name }}">{{ .Name }}</a></td>
					<td>{{ .CommaCount }}</td>
					<td><a href="render/{{ replace .Name "/" "_" }}.html">link</a></td>
				</tr>
				{{- end }}
			</tbody>
		</table>
		</div>
	</div>
</body>
</html>`
	parsedTemplate, err := template.
		New("index").
		Funcs(template.FuncMap{
			"replace": func(input, from, to string) string {
				return strings.ReplaceAll(input, from, to)
			},
		}).
		Parse(templateStr)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile("index.html", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0744)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	err = parsedTemplate.Execute(f, images)
	if err != nil {
		log.Fatal(err)
	}
}

func getPullCount(image string) {
	log.Printf("Scrape pull count for the image '%v'\n", image)
	res, err := http.Get(URLBase + image)
	if err != nil {
		log.Fatal(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var i Image
	json.Unmarshal(body, &i)
	i.Name = image
	i.CommaCount = humanize.Comma(i.Count)
	images = append(images, i)
}
