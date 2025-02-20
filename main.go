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
	yaml "gopkg.in/yaml.v3"
)

type List struct {
	Images []string `yaml:"images"`
	Sums   []struct {
		Name   string   `yaml:"name"`
		Images []string `yaml:"images"`
	} `yaml:"sums"`
}

type Image struct {
	Name       string
	Count      int `json:"pull_count"`
	HumanCount string
}

var (
	list   List
	images []Image
)

const (
	URLBase = "https://hub.docker.com/v2/repositories/"
)

func init() {
	images = make([]Image, 0)
}

func main() {
	l := flag.String("l", "", "YAML file with the list of images to track")
	dataFolder := flag.String("d", "./data", "Destination folder for .csv")
	renderFolder := flag.String("r", "./render", "Destination folder for the .html")
	flag.Parse()

	if *l == "" {
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

	yamlFile, err := os.ReadFile(*l)
	if err != nil {
		log.Fatalf("Read file: '%v'\n", err)
	}
	if err := yaml.Unmarshal(yamlFile, &list); err != nil {
		log.Fatalf("Unmarshal: '%v'\n", err)
	}

	for _, i := range list.Images {
		getPullCount(i)
	}

	for _, i := range list.Sums {
		s := Image{
			Name: "SUM/" + i.Name,
		}
		for _, j := range i.Images {
			for _, k := range images {
				if j == k.Name {
					s.Count += k.Count
				}
			}
		}
		s.HumanCount = humanize.Comma(int64(s.Count))
		images = append(images, s)
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
		charts.WithColorsOpts(opts.Colors{"blue", "orange"}),
	)

	line.ExtendYAxis(opts.YAxis{
		Name:  "delta",
		Type:  "value",
		Show:  opts.Bool(true),
		Scale: opts.Bool(true),
	})

	line.SetXAxis(xData)
	line.AddSeries("# pulls", yDataL)

	AddMarklines(line, baseName)

	line.AddSeries("delta", yDataR, charts.WithLineChartOpts(opts.LineChart{YAxisIndex: 1}))

	o, _ := os.Create(fmt.Sprintf("%v/%v.html", renderFolder, baseName))
	line.Render(o)
}

func AddMarklines(line *charts.Line, image string) {
	releases := make(map[string]string)
	switch image {
	case "falcosecurity_falcosidekick":
		releases = falcosidekick_versions()
	case "falcosecurity_falcosidekick-ui":
		releases = falcosidekick_ui_versions()
	case "falcosecurity_falcoctl":
		releases = falcoctl_versions()
	case "falcosecurity_falco", "falcosecurity_falco-no-driver", "falcosecurity_falco-driver-loader", "falcosecurity_falco-driver-loader-legacy":
		releases = falco_versions()
	case "SUM_falco", "SUM_falco-driver-loader":
		releases = falco_versions()
	}

	for date, version := range releases {
		AddMarkline(line, date, version)
	}
}

func AddMarkline(line *charts.Line, date, version string) {
	line.SetSeriesOptions(
		charts.WithMarkLineNameXAxisItemOpts(
			opts.MarkLineNameXAxisItem{
				Name:  version,
				XAxis: date,
			},
		),
		charts.WithMarkLineStyleOpts(opts.MarkLineStyle{
			Label: &opts.Label{
				Show:      opts.Bool(true),
				Formatter: "{b}",
			},
			LineStyle: &opts.LineStyle{Color: "gray"},
		}),
		charts.WithLineChartOpts(opts.LineChart{
			ShowSymbol: opts.Bool(true),
		}),
		charts.WithLabelOpts(opts.Label{
			Show: opts.Bool(false),
		}),
	)

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
	<div class="row flex">
		<div class="col s3">
		<table class="striped responsive-table" style="margin: 20px">
			<caption>IMAGES</caption>
			<thead>
				<tr>
					<th>Image</th>
					<th>Last count</th>
					<th>Chart</th>
				</tr>
			</thead>
			<tbody>
				{{- range . }}
				{{ if not (hasPrefix .Name "SUM") }}
					<tr>
						<td><a href="https://hub.docker.com/r/{{ .Name }}">{{ .Name }}</a></td>
						<td>{{ .HumanCount }}</td>
						<td><a href="render/{{ replace .Name "/" "_" }}.html"><span class="material-symbols-outlined">monitoring</span></a></td>
					</tr>
				{{ end }}
				{{- end }}
			</tbody>
		</table>
		</div>
		<div class="col s3">
		<table class="striped responsive-table" style="margin: 20px">
			<caption>SUMS</caption>
			<thead>
				<tr>
					<th>Image</th>
					<th>Last count</th>
					<th>Chart</th>
				</tr>
			</thead>
			<tbody>
				{{- range . }}
				{{ if (hasPrefix .Name "SUM") }}
					<tr>
						<td><a href="https://hub.docker.com/r/{{ .Name }}">{{ .Name }}</a></td>
						<td>{{ .HumanCount }}</td>
						<td><a href="render/{{ replace .Name "/" "_" }}.html"><span class="material-symbols-outlined">monitoring</span></a></td>
					</tr>
				{{ end }}
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
		Funcs(template.FuncMap{
			"hasPrefix": strings.HasPrefix,
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

func falcosidekick_versions() map[string]string {
	releases := map[string]string{
		"2018/10/10": "1.0.1",
		"2018/10/13": "1.0.2",
		"2019/01/30": "1.0.3",
		"2019/02/01": "1.0.4",
		"2019/04/09": "1.0.5",
		"2019/05/09": "1.0.7",
		"2019/05/10": "1.1.0",
		"2019/05/23": "2.0.0",
		"2019/06/12": "2.1.0",
		"2019/06/13": "2.2.0",
		"2019/06/17": "2.3.0",
		"2019/06/26": "2.4.0",
		"2019/08/12": "2.5.0",
		"2019/08/26": "2.6.0",
		"2019/08/27": "2.7.0",
		"2019/08/28": "2.7.2",
		"2019/09/11": "2.8.0",
		"2019/10/04": "2.9.0",
		"2019/10/07": "2.9.1",
		"2019/10/11": "2.9.2",
		"2019/10/18": "2.9.3",
		"2019/10/22": "2.10.0",
		"2019/11/13": "2.11.0",
		"2020/01/06": "2.11.1",
		"2020/01/16": "2.12.0",
		"2020/01/28": "2.12.1",
		"2020/04/21": "2.12.3",
		"2020/06/14": "2.13.0",
		"2020/08/08": "2.14.0",
		"2020/10/28": "2.15.0",
		"2020/10/29": "2.16.0",
		"2020/11/15": "2.17.0",
		"2020/11/20": "2.18.0",
		"2020/12/01": "2.19.0",
		"2020/12/02": "2.19.1",
		"2021/01/13": "2.20.0",
		"2021/02/12": "2.21.0",
		"2021/04/06": "2.22.0",
		"2021/06/22": "2.23.0",
		"2021/06/23": "2.23.1",
		"2021/08/16": "2.24.0",
		"2022/05/12": "2.25.0",
		"2022/06/17": "2.26.0",
		"2023/01/09": "2.27.0",
		"2023/07/27": "2.28.0",
		"2024/07/02": "2.29.0",
		"2024/11/27": "2.30.0",
		"2025/02/03": "2.31.0",
		"2025/02/04": "2.31.1",
	}
	return releases
}

func falco_versions() map[string]string {
	releases := map[string]string{
		"2023/06/07": "0.35.0",
		"2023/06/29": "0.35.1",
		"2023/09/26": "0.36.0",
		"2023/10/16": "0.36.1",
		"2023/10/27": "0.36.2",
		"2024/01/30": "0.37.0",
		"2024/02/13": "0.37.1",
		"2024/05/30": "0.38.0",
		"2024/06/19": "0.38.1",
		"2024/08/19": "0.38.2",
		"2024/10/01": "0.39.0",
		"2024/10/09": "0.39.1",
		"2024/11/21": "0.39.2",
		"2025/01/28": "0.40.0",
	}
	return releases
}

func falcoctl_versions() map[string]string {
	releases := map[string]string{
		"2019/09/04": "0.0.1",
		"2019/10/14": "0.0.2",
		"2019/10/22": "0.0.3",
		"2019/10/24": "0.0.4",
		"2019/10/28": "0.0.5",
		"2019/10/31": "0.0.6",
		"2019/11/07": "0.0.7",
		"2020/07/15": "0.1.0",
		"2023/02/02": "0.3.0",
		"2023/02/06": "0.4.0",
		"2023/05/23": "0.5.0",
		"2023/06/12": "0.5.1",
		"2023/09/04": "0.6.0",
		"2023/09/18": "0.6.1",
		"2023/09/22": "0.6.2",
		"2024/01/12": "0.7.0",
		"2024/01/23": "0.7.1",
		"2024/02/12": "0.7.2",
		"2024/02/22": "0.7.3",
		"2024/05/28": "0.8.0",
		"2024/08/01": "0.9.0",
		"2024/09/03": "0.9.1",
		"2024/09/16": "0.10.0",
		"2024/11/21": "0.10.1",
		"2025/01/27": "0.11.0",
	}
	return releases
}

func falcosidekick_ui_versions() map[string]string {
	releases := map[string]string{
		"2021/02/11": "0.1.0",
		"2021/02/26": "0.2.0",
		"2021/06/23": "1.1.0",
		"2022/05/11": "2.0.0",
		"2022/05/25": "2.0.1",
		"2022/06/05": "2.0.2",
		"2023/01/10": "2.1.0",
		"2023/09/14": "2.2.0",
	}
	return releases
}
