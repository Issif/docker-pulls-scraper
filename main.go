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
	"sort"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type Image struct {
	Name       string
	Count      int `json:"pull_count"`
	HumanCount string
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
	if err := listFileScanner.Err(); err != nil {
		log.Fatal(err)
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
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var delta int

	if os.IsNotExist(errorExist) {
		if _, err := f.WriteString("Date,Count,Delta\n"); err != nil {
			log.Fatal(err)
		}
	} else {
		var lastLine string
		fileScanner := bufio.NewScanner(f)
		for fileScanner.Scan() {
			if l := fileScanner.Text(); l != "" {
				lastLine = l
			}
		}
		if err := fileScanner.Err(); err != nil {
			log.Fatal(err)
		}
		lastCount, _ := strconv.Atoi(strings.Split(lastLine, ",")[1])
		delta = image.Count - lastCount
	}

	if _, err := f.WriteString(fmt.Sprintf("%v,%v,%v\n", time.Now().Format("2006/01/02"), image.Count, delta)); err != nil {
		log.Fatal(err)
	}
}

func renderChart(image Image, dataFolder, renderFolder string) {
	baseName := strings.ReplaceAll(image.Name, "/", "_")
	log.Printf("Writing of the .html for the image '%v' in '%v'\n", image.Name, fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	f, err := os.Open(fmt.Sprintf("%v/%v.csv", dataFolder, baseName))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	xData := make([]string, 0)
	yDataL := make([]opts.LineData, 0)
	yDataR := make([]opts.LineData, 0)

	fileScanner := bufio.NewScanner(f)
	fileScanner.Scan()
	for fileScanner.Scan() {
		s := fileScanner.Text()
		xData = append(xData, strings.Split(s, ",")[0])
		yDataL = append(yDataL, opts.LineData{Value: strings.Split(s, ",")[1]})
		yDataR = append(yDataR, opts.LineData{Value: strings.Split(s, ",")[2]})
	}
	if err := fileScanner.Err(); err != nil {
		log.Fatal(err)
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: image.Name,
			Width:     "100%",
			Height:    "95vh"}),
		charts.WithTitleOpts(opts.Title{
			Title: image.Name,
		}),
		charts.WithColorsOpts(opts.Colors{"#ff9999", "#00ff77"}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 0,
			End:   100,
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:         opts.Bool(true),
			SelectedMode: "multiple",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
				Snap: opts.Bool(true),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "# pulls",
			Type: "value",
			Show: opts.Bool(true),
		}),
	)

	line.ExtendYAxis(opts.YAxis{
		Name:  "delta",
		Type:  "value",
		Show:  opts.Bool(true),
		Scale: opts.Bool(true),
	})

	line.SetXAxis(xData)
	line.AddSeries("delta", yDataR, charts.WithLineChartOpts(opts.LineChart{YAxisIndex: 1}))
	line.AddSeries("# pulls", yDataL)
	o, _ := os.Create(fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	line.Render(o)
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
	<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@24,400,0,0" />
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
					<td>{{ .HumanCount }}</td>
					<td><a href="render/{{ replace .Name "/" "_" }}.html"><span class="material-symbols-outlined">monitoring</span></a></td>
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
	f, err := os.OpenFile("index.html", os.O_RDWR|os.O_CREATE, 0744)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	sort.Slice(images, func(i, j int) bool {
		return images[i].Count > images[j].Count
	})

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
	i.HumanCount = humanize.Comma(int64(i.Count))
	images = append(images, i)
}
